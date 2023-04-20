package rates

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/martian/log"
	"github.com/shopspring/decimal"
)

type TonRates struct { // the values are equated to the TON
	mu    sync.RWMutex
	Rates map[string]float64
}

func (r *TonRates) GetRates() map[string]float64 {
	return r.Rates
}

func InitTonRates() *TonRates {
	rates := &TonRates{Rates: map[string]float64{}}

	go func() {
		for {
			rates.refresh()
			time.Sleep(time.Minute * 5)
		}
	}()

	return rates
}

func (r *TonRates) refresh() {
	r.mu.Lock()
	defer r.mu.Unlock()
	rates, err := getRates()
	if err != nil {
		return
	}
	r.Rates = rates
}

func getRates() (map[string]float64, error) {
	okx := getOKXPrice()
	huobi := getHuobiPrice()

	if okx == 0 && huobi == 0 {
		return nil, fmt.Errorf("failed to get ton price")
	}

	meanTonPriceToUSD := (huobi + okx) / 2

	fiatPrices := getFiatPrices()
	pools := getPools()

	rates := make(map[string]float64)
	for currency, price := range fiatPrices {
		rates[currency] = meanTonPriceToUSD * price
	}
	for token, coinsCount := range pools {
		rates[token] = meanTonPriceToUSD * coinsCount
	}

	rates["TON"] = meanTonPriceToUSD

	return rates, nil
}

func getPools() map[string]float64 {
	resp, err := http.Get("https://api.dedust.io/v2/pools")
	if err != nil {
		log.Errorf("failed to fetch rates: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var respBody []Pool
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Errorf("failed to decode response: %v", err)
		return nil
	}

	mapOfPool := make(map[string]float64)
	for _, pool := range respBody {
		if len(pool.Assets) != 2 {
			log.Errorf("count of assets not 2")
			continue
		}
		firstAsset := pool.Assets[0]
		secondAsset := pool.Assets[1]

		if firstAsset.Metadata == nil || firstAsset.Metadata.Symbol != "TON" {
			continue
		}

		firstReserve := pool.Reserves[0]
		secondReserve := pool.Reserves[1]

		if firstReserve == "0" || secondReserve == "0" {
			continue
		}

		decimals := int32(9)
		if secondAsset.Metadata != nil && secondAsset.Metadata.Decimals != 0 {
			decimals = secondAsset.Metadata.Decimals
		}

		firstReserveConverted, _ := strconv.ParseFloat(firstReserve, 64)
		secondReserveConverted, _ := strconv.ParseFloat(secondReserve, 64)

		decimalFirstReserve := decimal.NewFromFloat(firstReserveConverted)
		decimalSecondReserve := decimal.NewFromFloat(secondReserveConverted)

		price, _ := decimalSecondReserve.Div(decimalFirstReserve).Round(decimals).Float64()

		mapOfPool[secondAsset.Address] = price
	}

	return mapOfPool
}

type Pool struct {
	Assets []struct {
		Type     string `json:"type"`
		Address  string `json:"address"`
		Metadata *struct {
			Name     string `json:"name"`
			Symbol   string `json:"symbol"`
			Decimals int32  `json:"decimals"`
		} `json:"metadata"`
	} `json:"assets"`
	Reserves []string `json:"reserves"`
}

type huobiPrice struct {
	Status string `json:"status"`
	Tick   struct {
		Data []struct {
			Ts     int64   `json:"ts"`
			Amount float64 `json:"amount"`
			Price  float64 `json:"price"`
		} `json:"data"`
	} `json:"tick"`
}

type okxPrice struct {
	Code string `json:"code"`
	Data []struct {
		Last string `json:"last"`
	} `json:"data"`
}

func getHuobiPrice() float64 {
	resp, err := http.Get("https://api.huobi.pro/market/trade?symbol=tonusdt")
	if err != nil {
		log.Errorf("can't load huobi price")
		return 0
	}
	defer resp.Body.Close()

	var respBody huobiPrice
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

	var respBody okxPrice
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
