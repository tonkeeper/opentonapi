package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/labstack/gommon/log"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tep64"
	"github.com/tonkeeper/tongo/ton"
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
	stonfiPools := getStonFiPool(meanTonPriceToUSD)
	for address, price := range stonfiPools {
		if _, ok := pools[address]; !ok {
			pools[address] = price
		}
	}
	megatonPool := getMegatonPool(meanTonPriceToUSD)
	for address, price := range megatonPool {
		if _, ok := pools[address]; !ok {
			pools[address] = price
		}
	}
	tonstakersJetton, tonstakersPrice, err := getTonstakersPrice(references.TonstakersAccountPool)
	if err == nil {
		pools[tonstakersJetton.ToHuman(true, false)] = tonstakersPrice
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

func getMegatonPool(tonPrice float64) map[string]float64 {
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
	var respBody []struct {
		Address string  `json:"address"`
		Price   float64 `json:"price"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Errorf("failed to decode response: %v", err)
		return map[string]float64{}
	}

	mapOfPool := make(map[string]float64)
	for _, pool := range respBody {
		mapOfPool[pool.Address] = pool.Price / tonPrice
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

	var respBody struct {
		AssetList []struct {
			ContractAddress string  `json:"contract_address"`
			DexUsdPrice     *string `json:"dex_usd_price"`
		} `json:"asset_list"`
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
		if jettonPrice, err := strconv.ParseFloat(*pool.DexUsdPrice, 64); err == nil {
			mapOfPool[pool.ContractAddress] = jettonPrice / tonPrice
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

	mapOfPool := make(map[string]float64)
	for _, pool := range respBody {
		if len(pool.Assets) != 2 || len(pool.Reserves) != 2 {
			continue
		}

		firstAsset, secondAsset := pool.Assets[0], pool.Assets[1]
		if firstAsset.Metadata == nil || firstAsset.Metadata.Symbol != "TON" {
			continue
		}

		firstReserve, err := strconv.ParseFloat(pool.Reserves[0], 64)
		if err != nil {
			continue
		}
		secondReserve, err := strconv.ParseFloat(pool.Reserves[1], 64)
		if err != nil {
			continue
		}
		if firstReserve < float64(100*ton.OneTON) || secondReserve < float64(100*ton.OneTON) {
			continue
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

		// TODO: change algorithm math price for other type pool (volatile/stable)
		price := 1 / ((secondReserve / math.Pow(10, secondReserveDecimals)) / (firstReserve / math.Pow(10, 9)))
		mapOfPool[secondAsset.Address] = price
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
		if rateConverted, err := strconv.ParseFloat(rate, 64); err == nil {
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
			JettonMinter    string `json:"jetton_minter"`
			ProjectBalance  int64  `json:"projected_balance"`
			ProjectedSupply int64  `json:"projected_supply"`
		}
	}
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Errorf("failed to decode response: %v", err)
		return tongo.AccountID{}, 0, err
	}

	if !respBody.Success {
		return tongo.AccountID{}, 0, fmt.Errorf("failed success")
	}
	if respBody.Decoded.ProjectBalance == 0 || respBody.Decoded.ProjectedSupply == 0 {
		return tongo.AccountID{}, 0, fmt.Errorf("empty balance")
	}
	accountJetton, err := tongo.ParseAccountID(respBody.Decoded.JettonMinter)
	if err != nil {
		return tongo.AccountID{}, 0, err
	}
	price := float64(respBody.Decoded.ProjectBalance) / float64(respBody.Decoded.ProjectedSupply)

	return accountJetton, price, nil
}
