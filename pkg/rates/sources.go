package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"

	"github.com/labstack/gommon/log"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tep64"
)

type storage interface {
	GetJettonMasterMetadata(ctx context.Context, master tongo.AccountID) (tep64.Metadata, error)
}

func (m *Mock) GetCurrentRates() (map[string]float64, error) {
	rates := make(map[string]float64)

	fiatPrices := getCoinbaseFiatPrices()
	for currency, rate := range getExchangerateFiatPrices() {
		if _, ok := fiatPrices[currency]; !ok {
			fiatPrices[currency] = rate
		}
	}
	tonPriceOKX := getTonOKXPrice()
	tonPriceHuobi := getTonHuobiPrice()
	if tonPriceOKX == 0 && tonPriceHuobi == 0 {
		return nil, fmt.Errorf("failed to get ton price")
	}
	meanTonPriceToUSD := (tonPriceHuobi + tonPriceOKX) / 2

	pools := m.getDedustPool()
	megatonPool := getMegatonPool()
	for address, price := range megatonPool {
		if _, ok := pools[address]; !ok {
			pools[address] = price
		}
	}
	stonfiPools := getStonFiPool(meanTonPriceToUSD)
	for address, price := range stonfiPools {
		if _, ok := pools[address]; !ok {
			pools[address] = price
		}
	}
	tonstakersJetton, tonstakersPrice, err := getTonstakersPrice(references.TonstakersAccountPool)
	if err == nil {
		rates[tonstakersJetton.ToHuman(true, false)] = tonstakersPrice
	}

	rates["TON"] = 1
	for currency, price := range fiatPrices {
		rates[currency] = meanTonPriceToUSD * price
	}
	for token, coinsCount := range pools {
		rates[token] = coinsCount
	}

	return rates, nil
}

func getMegatonPool() map[string]float64 {
	resp, err := http.Get("https://megaton.fi/api/token/infoList")
	if err != nil {
		log.Errorf("failed to fetch megaton rates: %v", err)
		return map[string]float64{}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Errorf("invalid status code megaton rates: %v", resp.StatusCode)
		return map[string]float64{}
	}
	type Pool struct {
		Address string  `json:"address"`
		Price   float64 `json:"price"`
	}
	var respBody []Pool
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Errorf("failed to decode response: %v", err)
		return map[string]float64{}
	}

	mapOfPool := make(map[string]float64)
	for _, pool := range respBody {
		if pool.Price != 0 {
			mapOfPool[pool.Address] = 1 / pool.Price
		}
	}

	return mapOfPool
}

func getStonFiPool(tonPrice float64) map[string]float64 {
	resp, err := http.Get("https://api.ston.fi/v1/assets")
	if err != nil {
		log.Errorf("failed to fetch stonfi rates: %v", err)
		return map[string]float64{}
	}
	defer resp.Body.Close()

	type Pool struct {
		ContractAddress string  `json:"contract_address"`
		DexUsdPrice     *string `json:"dex_usd_price"`
	}

	var respBody struct {
		AssetList []Pool `json:"asset_list"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Errorf("failed to decode response: %v", err)
		return map[string]float64{}
	}

	mapOfPool := make(map[string]float64)
	for _, pool := range respBody.AssetList {
		if pool.DexUsdPrice == nil {
			continue
		}
		price, err := strconv.ParseFloat(*pool.DexUsdPrice, 64)
		if err == nil && price != 0 {
			mapOfPool[pool.ContractAddress] = tonPrice / price
		}
	}

	return mapOfPool
}

func (m *Mock) getDedustPool() map[string]float64 {
	resp, err := http.Get("https://api.dedust.io/v2/pools")
	if err != nil {
		log.Errorf("failed to fetch dedust rates: %v", err)
		return map[string]float64{}
	}
	defer resp.Body.Close()

	type Pool struct {
		TotalSupply string `json:"totalSupply"`
		Assets      []struct {
			Address  string `json:"address"`
			Metadata *struct {
				Symbol   string  `json:"symbol"`
				Decimals float64 `json:"decimals"`
			} `json:"metadata"`
		} `json:"assets"`
		Reserves []string `json:"reserves"`
	}

	var respBody []Pool
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Errorf("failed to decode response: %v", err)
		return map[string]float64{}
	}

	var wg sync.WaitGroup
	chanMapOfPool := make(chan map[string]float64)
	for _, pool := range respBody {
		wg.Add(1)

		go func(pool Pool) {
			defer wg.Done()

			if pool.TotalSupply < "1000000000" || len(pool.Assets) != 2 {
				return
			}

			firstAsset, secondAsset := pool.Assets[0], pool.Assets[1]
			if firstAsset.Metadata == nil || firstAsset.Metadata.Symbol != "TON" {
				return
			}
			firstReserve, secondReserve := pool.Reserves[0], pool.Reserves[1]
			if firstReserve == "0" || secondReserve == "0" {
				return
			}

			secondReserveDecimals := float64(9)
			if secondAsset.Metadata == nil || secondAsset.Metadata.Decimals != 0 {
				accountID, _ := tongo.ParseAccountID(secondAsset.Address)
				meta, err := m.Storage.GetJettonMasterMetadata(context.Background(), accountID)
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

func getTonHuobiPrice() float64 {
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

func getTonOKXPrice() float64 {
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

func getExchangerateFiatPrices() map[string]float64 {
	resp, err := http.Get("https://api.exchangerate.host/latest?base=USD")
	if err != nil {
		log.Errorf("can't load exchangerate prices")
		return map[string]float64{}
	}
	defer resp.Body.Close()

	var respBody struct {
		Rates map[string]float64 `json:"rates"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Errorf("failed to decode response: %v", err)
		return map[string]float64{}
	}

	mapOfPrices := make(map[string]float64)
	for currency, rate := range respBody.Rates {
		mapOfPrices[currency] = rate
	}

	return mapOfPrices
}

func getCoinbaseFiatPrices() map[string]float64 {
	resp, err := http.Get("https://api.coinbase.com/v2/exchange-rates?currency=USD")
	if err != nil {
		log.Errorf("can't load coinbase prices")
		return map[string]float64{}
	}
	defer resp.Body.Close()

	var respBody struct {
		Data struct {
			Rates map[string]string `json:"rates"`
		} `json:"data"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Errorf("failed to decode response: %v", err)
		return map[string]float64{}
	}

	mapOfPrices := make(map[string]float64)
	for currency, rate := range respBody.Data.Rates {
		rateConverted, err := strconv.ParseFloat(rate, 64)
		if err == nil {
			mapOfPrices[currency] = rateConverted
		}
	}

	return mapOfPrices
}

// getTonstakersPrice is used to retrieve the price and token address of an account on the Tonstakers pool.
// We are using the TonApi, because the standard liteserver executor is incapable of invoking methods on the account
func getTonstakersPrice(pool tongo.AccountID) (tongo.AccountID, float64, error) {
	resp, err := http.Get(fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/get_pool_full_data", pool.ToRaw()))
	if err != nil {
		log.Errorf("can't load tonstakers price")
		return tongo.AccountID{}, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return tongo.AccountID{}, 0, fmt.Errorf("bad status code: %v", resp.StatusCode)
	}
	var respBody struct {
		Success bool `json:"success"`
		Decoded struct {
			JettonMinter string `json:"jetton_minter"`
			TotalBalance int64  `json:"total_balance"`
			Supply       int64  `json:"supply"`
		}
	}
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Errorf("failed to decode response: %v", err)
		return tongo.AccountID{}, 0, err
	}

	if !respBody.Success {
		return tongo.AccountID{}, 0, fmt.Errorf("failed success")
	}
	if respBody.Decoded.Supply == 0 || respBody.Decoded.TotalBalance == 0 {
		return tongo.AccountID{}, 0, fmt.Errorf("empty balance")
	}
	accountJetton, err := tongo.ParseAccountID(respBody.Decoded.JettonMinter)
	if err != nil {
		return tongo.AccountID{}, 0, err
	}
	price := float64(respBody.Decoded.TotalBalance) / float64(respBody.Decoded.Supply)

	return accountJetton, price, nil
}
