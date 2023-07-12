package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/martian/log"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteapi"
)

type TonRates struct { // the values are equated to the TON
	mu             sync.RWMutex
	executor       *liteapi.Client
	TodayRates     map[string]float64
	YesterdayRates map[string]float64
}

func (r *TonRates) GetTodayRates() map[string]float64 {
	return r.TodayRates
}

func (r *TonRates) GetYesterdayRates() map[string]float64 {
	return r.YesterdayRates
}

func InitTonRates(historicalPriceUrl string) *TonRates {
	executor, err := liteapi.NewClientWithDefaultMainnet()
	if err != nil {
		panic(fmt.Sprintf("failed init ton rates: %v", err))
	}
	rates := &TonRates{
		TodayRates:     map[string]float64{},
		YesterdayRates: map[string]float64{},
		executor:       executor,
	}

	go func() {
		for {
			rates.refresh(historicalPriceUrl)
			time.Sleep(time.Minute * 5)
		}
	}()

	return rates
}

func (r *TonRates) refresh(historicalPriceUrl string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	todayRates, err := r.getTodayRates()
	if err != nil {
		return
	}
	r.TodayRates = todayRates

	if historicalPriceUrl == "" {
		return
	}
	yesterdayRates, err := r.getYesterdayRates(historicalPriceUrl)
	if err != nil {
		return
	}
	r.YesterdayRates = yesterdayRates
}

func (r *TonRates) getTodayRates() (map[string]float64, error) {
	okx := getOKXPrice()
	huobi := getHuobiPrice()

	if okx == 0 && huobi == 0 {
		return nil, fmt.Errorf("failed to get ton price")
	}

	meanTonPriceToUSD := (huobi + okx) / 2

	fiatPrices := getFiatPrices()
	pools := r.getPools()

	rates := make(map[string]float64)
	for currency, price := range fiatPrices {
		rates[currency] = meanTonPriceToUSD * price
	}
	for token, coinsCount := range pools {
		rates[token] = coinsCount
	}

	rates["TON"] = 1

	return rates, nil
}

func (r *TonRates) getYesterdayRates(url string) (map[string]float64, error) {
	resp, err := http.Get(url)
	if err != nil {
		log.Errorf("failed to fetch yesterday rates: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	var respBody struct {
		Rates struct {
			TodayRates     map[string]float64 `json:"today_rates"`
			YesterdayRates map[string]float64 `json:"yesterday_rates"`
		} `json:"rates"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Errorf("failed to decode response: %v", err)
		return nil, err
	}

	return respBody.Rates.YesterdayRates, nil
}

func (r *TonRates) getPools() map[string]float64 {
	dedustPool := r.getDedustPool()
	redoubtPool := getRedoubtPool()
	for address, price := range redoubtPool {
		dedustPool[address] = price
	}
	return dedustPool
}

func getRedoubtPool() map[string]float64 {
	resp, err := http.Get("https://api.redoubt.online/v2/feed/jettons")
	if err != nil {
		log.Errorf("failed to fetch redoubt rates: %v", err)
		return nil
	}
	defer resp.Body.Close()

	type Pool struct {
		Name    string `json:"name"`
		Address string `json:"address"`
		Price   struct {
			Ton float64 `json:"ton"`
		} `json:"price"`
	}

	var respBody struct {
		Jettons []Pool `json:"jettons"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Errorf("failed to decode response: %v", err)
		return nil
	}

	mapOfPool := make(map[string]float64)
	for _, pool := range respBody.Jettons {
		mapOfPool[pool.Address] = 1 / pool.Price.Ton
	}

	return mapOfPool
}

func (r *TonRates) getStonFiPool(tonPrice float64) map[string]float64 {
	resp, err := http.Get("https://api.ston.fi/v1/assets")
	if err != nil {
		log.Errorf("failed to fetch stonfi rates: %v", err)
		return nil
	}
	defer resp.Body.Close()

	type Pool struct {
		ContractAddress string  `json:"contract_address"`
		Symbol          string  `json:"symbol"`
		UsdPrice        *string `json:"usd_price"`
		Decimals        int32   `json:"decimals"`
	}

	var respBody struct {
		AssetList []Pool `json:"asset_list"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Errorf("failed to decode response: %v", err)
		return nil
	}

	mapOfPool := make(map[string]float64)
	for _, pool := range respBody.AssetList {
		if pool.UsdPrice == nil {
			continue
		}
		price, err := strconv.ParseFloat(*pool.UsdPrice, 64)
		if err != nil {
			log.Errorf("failed to convert stonfi price: %v", err)
			continue
		}
		mapOfPool[pool.ContractAddress] = tonPrice / price
	}

	return mapOfPool
}

func (r *TonRates) getDedustPool() map[string]float64 {
	resp, err := http.Get("https://api.dedust.io/v2/pools")
	if err != nil {
		log.Errorf("failed to fetch dedust rates: %v", err)
		return nil
	}
	defer resp.Body.Close()

	type Pool struct {
		Assets []struct {
			Type     string `json:"type"`
			Address  string `json:"address"`
			Metadata *struct {
				Name     string  `json:"name"`
				Symbol   string  `json:"symbol"`
				Decimals float64 `json:"decimals"`
			} `json:"metadata"`
		} `json:"assets"`
		Reserves []string `json:"reserves"`
	}

	var respBody []Pool
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Errorf("failed to decode response: %v", err)
		return nil
	}

	var wg sync.WaitGroup
	chanMapOfPool := make(chan map[string]float64)
	for _, pool := range respBody {
		wg.Add(1)

		go func(pool Pool) {
			defer wg.Done()

			if len(pool.Assets) != 2 {
				log.Errorf("count of assets not 2")
				return
			}
			firstAsset := pool.Assets[0]
			secondAsset := pool.Assets[1]

			if firstAsset.Metadata == nil || firstAsset.Metadata.Symbol != "TON" {
				return
			}

			firstReserve := pool.Reserves[0]
			secondReserve := pool.Reserves[1]

			if firstReserve == "0" || secondReserve == "0" {
				return
			}

			secondReserveDecimals := float64(9)
			if secondAsset.Metadata == nil || secondAsset.Metadata.Decimals != 0 {
				accountID, _ := tongo.ParseAccountID(secondAsset.Address)
				meta, err := r.executor.GetJettonData(context.Background(), accountID)
				if err == nil && meta.Decimals != "" {
					decimals, err := strconv.Atoi(meta.Decimals)
					if err == nil {
						secondReserveDecimals = float64(decimals)
					}
				}
			}

			firstReserveConverted, _ := strconv.ParseFloat(firstReserve, 64)
			secondReserveConverted, _ := strconv.ParseFloat(secondReserve, 64)

			price := (secondReserveConverted / math.Pow(10, secondReserveDecimals)) / (firstReserveConverted / math.Pow(10, 9))
			chanMapOfPool <- map[string]float64{secondAsset.Address: price}
		}(pool)
	}

	go func() {
		wg.Wait()
		close(chanMapOfPool)
	}()

	mapOfPool := make(map[string]float64)
	for pools := range chanMapOfPool {
		for address, price := range pools {
			mapOfPool[address] = price
		}
	}

	return mapOfPool
}

func getHuobiPrice() float64 {
	resp, err := http.Get("https://api.huobi.pro/market/trade?symbol=tonusdt")
	if err != nil {
		log.Errorf("can't load huobi price")
		return 0
	}
	defer resp.Body.Close()

	var respBody struct {
		Status string `json:"status"`
		Tick   struct {
			Data []struct {
				Ts     int64   `json:"ts"`
				Amount float64 `json:"amount"`
				Price  float64 `json:"price"`
			} `json:"data"`
		} `json:"tick"`
	}

	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Errorf("failed to decode response: %v", err)
		return 0
	}

	if respBody.Status != "ok" {
		log.Errorf("failed to get huobi price: %v", err)
		return 0
	}

	if len(respBody.Tick.Data) == 0 {
		log.Errorf("invalid price")
		return 0
	}

	return respBody.Tick.Data[0].Price
}

func getOKXPrice() float64 {
	resp, err := http.Get("https://www.okx.com/api/v5/market/ticker?instId=TON-USDT")
	if err != nil {
		log.Errorf("can't load okx price")
		return 0
	}
	defer resp.Body.Close()

	var respBody struct {
		Code string `json:"code"`
		Data []struct {
			Last string `json:"last"`
		} `json:"data"`
	}

	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Errorf("failed to decode response: %v", err)
		return 0
	}

	if respBody.Code != "0" {
		log.Errorf("failed to get okx price: %v", err)
		return 0
	}

	if len(respBody.Data) == 0 {
		log.Errorf("invalid price")
		return 0
	}

	price, err := strconv.ParseFloat(respBody.Data[0].Last, 64)
	if err != nil {
		log.Errorf("invalid price")
		return 0
	}

	return price
}

func getFiatPrices() map[string]float64 {
	resp, err := http.Get("https://api.coinbase.com/v2/exchange-rates?currency=USD")
	if err != nil {
		log.Errorf("can't load coinbase prices")
		return nil
	}
	defer resp.Body.Close()

	var respBody struct {
		Data struct {
			Rates map[string]string `json:"rates"`
		} `json:"data"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Errorf("failed to decode response: %v", err)
		return nil
	}

	mapOfPrices := make(map[string]float64)
	for currency, rate := range respBody.Data.Rates {
		rateConverted, err := strconv.ParseFloat(rate, 64)
		if err != nil {
			log.Errorf("failed to convert str to float64 %v, err: %v", rate, err)
			continue
		}
		mapOfPrices[currency] = rateConverted
	}

	return mapOfPrices
}
