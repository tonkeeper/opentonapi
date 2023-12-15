package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/labstack/gommon/log"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tep64"
	"github.com/tonkeeper/tongo/ton"
)

var errorsCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "rates_getter_errors_total",
}, []string{"source"})

type storage interface {
	GetJettonMasterMetadata(ctx context.Context, master tongo.AccountID) (tep64.Metadata, error)
}

func (m *Mock) GetCurrentRates() (map[string]float64, error) {
	rates := make(map[string]float64)
	const tonstakers string = "tonstakers"

	marketsPrice := m.GetCurrentMarketsTonPrice()
	medianTonPriceToUsd, err := getMedianTonPrice(marketsPrice)
	if err != nil {
		return rates, err
	}

	fiatPrices := getFiatPrices()
	pools := getPools(medianTonPriceToUsd, m.Storage)

	for attempt := 0; attempt < 3; attempt++ {
		if tonstakersJetton, tonstakersPrice, err := getTonstakersPrice(references.TonstakersAccountPool); err == nil {
			pools[tonstakersJetton] = tonstakersPrice
			break
		}
		errorsCounter.WithLabelValues(tonstakers).Inc()
		time.Sleep(time.Second * 3)
	}

	// All data is displayed to the ratio to TON
	// For example: 1 Jetton = ... TON, 1 USD = ... TON
	rates["TON"] = 1
	for currency, price := range fiatPrices {
		if price != 0 {
			rates[currency] = 1 / (price * medianTonPriceToUsd)
		}
	}
	for token, coinsCount := range pools {
		rates[token.ToRaw()] = coinsCount
	}

	return rates, nil
}

func getMedianTonPrice(marketsPrice []Market) (float64, error) {
	var prices []float64
	for _, market := range marketsPrice {
		prices = append(prices, market.UsdPrice)
	}
	sort.Float64s(prices)

	length := len(prices)
	if length%2 == 0 { // if the length of the array is even, take the average of the two middle elements
		middle1 := prices[length/2-1]
		middle2 := prices[length/2]
		return (middle1 + middle2) / 2, nil
	}

	// if the length of the array is odd, return the middle element.
	return prices[length/2], nil
}

func getPools(tonPrice float64, storage storage) map[ton.AccountID]float64 {
	const (
		dedust  string = "dedust"
		stonfi  string = "stonfi"
		megaton string = "megaton"
	)
	markets := []Market{
		{
			Name:   dedust,
			ApiURL: "https://api.dedust.io/v2/pools",
		},
		{
			Name:   stonfi,
			ApiURL: "https://api.ston.fi/v1/assets",
		},
		{
			Name:   megaton,
			ApiURL: "https://megaton.fi/api/token/infoList",
		},
	}
	pools := make(map[ton.AccountID]float64)
	respBody, err := sendRequest(markets[0].ApiURL)
	if err != nil {
		errorsCounter.WithLabelValues(dedust).Inc()
	}
	if respBody != nil {
		pools = convertedDedustPoolResponse(storage, respBody)
	}
	for _, market := range markets {
		respBody, err = sendRequest(market.ApiURL)
		if err != nil {
			continue
		}
		switch market.Name {
		case stonfi:
			stonfiPools := convertedStonFiPoolResponse(tonPrice, respBody)
			if len(stonfiPools) == 0 {
				errorsCounter.WithLabelValues(stonfi).Inc()
			}
			for currency, rate := range stonfiPools {
				if _, ok := pools[currency]; !ok {
					pools[currency] = rate
				}
			}
		case megaton:
			megatonPools := convertedMegatonPoolResponse(tonPrice, respBody)
			if len(megatonPools) == 0 {
				errorsCounter.WithLabelValues(megaton).Inc()
			}
			for currency, rate := range megatonPools {
				if _, ok := pools[currency]; !ok {
					pools[currency] = rate
				}
			}
		}
	}
	return pools
}

func getFiatPrices() map[string]float64 {
	const (
		coinbase     string = "coinbase"
		exchangerate string = "exchangerate"
	)
	markets := []Market{
		{
			Name:   coinbase,
			ApiURL: "https://api.coinbase.com/v2/exchange-rates?currency=USD",
		},
		{
			Name:   exchangerate,
			ApiURL: "https://api.exchangerate.host/latest?base=USD",
		},
	}
	prices := make(map[string]float64)
	for _, market := range markets {
		respBody, err := sendRequest(market.ApiURL)
		if err != nil {
			continue
		}
		switch market.Name {
		case coinbase:
			for currency, rate := range convertedCoinBaseFiatPricesResponse(respBody) {
				if _, ok := prices[currency]; !ok {
					prices[currency] = rate
				}
			}
		case exchangerate:
			for currency, rate := range convertedExchangerateFiatPricesResponse(respBody) {
				if _, ok := prices[currency]; !ok {
					prices[currency] = rate
				}
			}
		}
	}
	return prices
}

func convertedMegatonPoolResponse(tonPrice float64, respBody []byte) map[ton.AccountID]float64 {
	var data []struct {
		Address string  `json:"address"`
		Price   float64 `json:"price"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		return map[ton.AccountID]float64{}
	}
	pools := make(map[ton.AccountID]float64)
	for _, pool := range data {
		account, err := tongo.ParseAddress(pool.Address)
		if err != nil {
			continue
		}
		pools[account.ID] = pool.Price / tonPrice
	}
	return pools
}

func convertedStonFiPoolResponse(tonPrice float64, respBody []byte) map[ton.AccountID]float64 {
	var data struct {
		AssetList []struct {
			ContractAddress string  `json:"contract_address"`
			DexUsdPrice     *string `json:"dex_usd_price"`
		} `json:"asset_list"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		return map[ton.AccountID]float64{}
	}
	pools := make(map[ton.AccountID]float64)
	for _, pool := range data.AssetList {
		if pool.DexUsdPrice == nil {
			continue
		}
		account, err := tongo.ParseAddress(pool.ContractAddress)
		if err != nil {
			continue
		}
		if jettonPrice, err := strconv.ParseFloat(*pool.DexUsdPrice, 64); err == nil {
			pools[account.ID] = jettonPrice / tonPrice
		}
	}
	return pools
}

func convertedDedustPoolResponse(storage storage, respBody []byte) map[ton.AccountID]float64 {
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
	var data []Pool
	if err := json.Unmarshal(respBody, &data); err != nil {
		return map[ton.AccountID]float64{}
	}
	pools := make(map[ton.AccountID]float64)
	for _, pool := range data {
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
			account, _ := tongo.ParseAddress(secondAsset.Address)
			meta, err := storage.GetJettonMasterMetadata(context.Background(), account.ID)
			if err == nil && meta.Decimals != "" {
				decimals, err := strconv.Atoi(meta.Decimals)
				if err == nil {
					secondReserveDecimals = float64(decimals)
				}
			}
		}
		account, err := tongo.ParseAddress(secondAsset.Address)
		if err != nil {
			continue
		}
		// TODO: change algorithm math price for other type pool (volatile/stable)
		price := 1 / ((secondReserve / math.Pow(10, secondReserveDecimals)) / (firstReserve / math.Pow(10, 9)))
		pools[account.ID] = price
	}
	return pools
}

func convertedExchangerateFiatPricesResponse(respBody []byte) map[string]float64 {
	var data struct {
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		log.Errorf("[convertedExchangerateFiatPricesResponse] failed to decode: %v", err)
		return map[string]float64{}
	}
	prices := make(map[string]float64)
	for currency, rate := range data.Rates {
		prices[currency] = rate
	}
	return prices
}

func convertedCoinBaseFiatPricesResponse(respBody []byte) map[string]float64 {
	var data struct {
		Data struct {
			Rates map[string]string `json:"rates"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		log.Errorf("[convertedCoinBaseFiatPricesResponse] failed to decode: %v", err)
		return map[string]float64{}
	}
	prices := make(map[string]float64)
	for currency, rate := range data.Data.Rates {
		if rateConverted, err := strconv.ParseFloat(rate, 64); err == nil {
			prices[currency] = rateConverted
		}
	}
	return prices
}

// getTonstakersPrice is used to retrieve the price and token address of an account on the Tonstakers pool.
// We are using the TonApi, because the standard liteserver executor is incapable of invoking methods on the account
func getTonstakersPrice(pool tongo.AccountID) (tongo.AccountID, float64, error) {
	resp, err := http.Get(fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/get_pool_full_data", pool.ToRaw()))
	if err != nil {
		log.Errorf("[getTonstakersPrice] can't load price")
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
		log.Errorf("[getTonstakersPrice] failed to decode response: %v", err)
		return tongo.AccountID{}, 0, err
	}

	if !respBody.Success {
		return tongo.AccountID{}, 0, fmt.Errorf("failed success")
	}
	if respBody.Decoded.ProjectBalance == 0 || respBody.Decoded.ProjectedSupply == 0 {
		return tongo.AccountID{}, 0, fmt.Errorf("empty balance")
	}
	accountJetton, err := tongo.ParseAddress(respBody.Decoded.JettonMinter)
	if err != nil {
		return tongo.AccountID{}, 0, err
	}
	price := float64(respBody.Decoded.ProjectBalance) / float64(respBody.Decoded.ProjectedSupply)

	return accountJetton.ID, price, nil
}
