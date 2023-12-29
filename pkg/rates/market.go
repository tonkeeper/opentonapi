package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/labstack/gommon/log"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
)

const (
	bitfinex     string = "Bitfinex"
	gateio       string = "Gate.io"
	bybit        string = "Bybit"
	kucoin       string = "KuCoin"
	okx          string = "OKX"
	huobi        string = "Huobi"
	dedust       string = "DeDust"
	stonfi       string = "STON.fi"
	megaton      string = "Megaton"
	coinbase     string = "Coinbase"
	exchangerate string = "Exchangerate"
)

type Market struct {
	ID                    int64
	Name                  string
	UsdPrice              float64
	ApiURL                string
	TonPriceConverter     func(closer io.ReadCloser) (float64, error)
	FiatPriceConverter    func(closer io.ReadCloser) map[string]float64
	PoolResponseConverter func(storage storage, tonPrice float64, respBody io.ReadCloser) map[ton.AccountID]float64
	DateUpdate            time.Time
}

func (m *Mock) GetCurrentMarketsTonPrice() []Market {
	now := time.Now()
	markets := []Market{
		{
			ID:                1,
			Name:              bitfinex,
			ApiURL:            "https://api-pub.bitfinex.com/v2/ticker/tTONUSD",
			TonPriceConverter: convertedTonBitFinexResponse,
			DateUpdate:        now,
		},
		{
			ID:                2,
			Name:              gateio,
			ApiURL:            "https://api.gateio.ws/api/v4/spot/tickers?currency_pair=TON_USDT",
			TonPriceConverter: convertedTonGateIOResponse,
			DateUpdate:        now,
		},
		{
			ID:                3,
			Name:              bybit,
			ApiURL:            "https://api.bybit.com/derivatives/v3/public/tickers?symbol=TONUSDT",
			TonPriceConverter: convertedTonBybitResponse,
			DateUpdate:        now,
		},
		{
			ID:                4,
			Name:              kucoin,
			ApiURL:            "https://www.kucoin.com/_api/trade-front/market/getSymbolTick?symbols=TON-USDT",
			TonPriceConverter: convertedTonKuCoinResponse,
			DateUpdate:        now,
		},
		{
			ID:                5,
			Name:              okx,
			ApiURL:            "https://www.okx.com/api/v5/market/ticker?instId=TON-USDT",
			TonPriceConverter: convertedTonOKXResponse,
			DateUpdate:        now,
		},
		{
			ID:                6,
			Name:              huobi,
			ApiURL:            "https://api.huobi.pro/market/trade?symbol=tonusdt",
			TonPriceConverter: convertedTonHuobiResponse,
			DateUpdate:        now,
		},
	}
	for idx, market := range markets {
		respBody, err := sendRequest(market.ApiURL)
		if err != nil {
			continue
		}
		market.UsdPrice, err = market.TonPriceConverter(respBody)
		if err != nil || market.UsdPrice == 0 {
			continue
		}
		markets[idx] = market
	}
	sort.Slice(markets, func(i, j int) bool {
		return markets[i].ID > markets[j].ID
	})
	return markets
}

func sendRequest(url string) (io.ReadCloser, error) {
	resp, err := http.Get(url)
	if err != nil {
		log.Errorf("[sendRequest] failed to get market price: %v", url)
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		log.Errorf("[sendRequest] failed to get market price: %v, %v", resp.StatusCode, url)
		return nil, fmt.Errorf("bad status code: %v", resp.StatusCode)
	}
	return resp.Body, nil
}

func convertedTonGateIOResponse(respBody io.ReadCloser) (float64, error) {
	var data []struct {
		Last string `json:"last"`
	}
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {
		log.Errorf("[convertedTonGateIOResponse] failed to decode response: %v", err)
		return 0, err
	}
	if len(data) == 0 {
		log.Errorf("[convertedTonGateIOResponse] empty data")
		return 0, fmt.Errorf("empty data")
	}
	price, err := strconv.ParseFloat(data[0].Last, 64)
	if err != nil {
		log.Errorf("[convertedTonGateIOResponse] failed to parse price: %v", err)
		return 0, fmt.Errorf("failed to parse price")
	}
	return price, nil
}

func convertedTonBybitResponse(respBody io.ReadCloser) (float64, error) {
	var data struct {
		RetMsg string `json:"retMsg"`
		Result struct {
			List []struct {
				LastPrice string `json:"lastPrice"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {
		log.Errorf("[convertedTonBybitResponse] failed to decode response: %v", err)
		return 0, err
	}
	if data.RetMsg != "OK" {
		log.Errorf("[convertedTonBybitResponse] unsuccessful response")
		return 0, fmt.Errorf("unsuccessful response")
	}
	if len(data.Result.List) == 0 {
		log.Errorf("[convertedTonBybitResponse] empty data")
		return 0, fmt.Errorf("empty data")
	}
	price, err := strconv.ParseFloat(data.Result.List[0].LastPrice, 64)
	if err != nil {
		log.Errorf("[convertedTonBybitResponse] failed to parse price: %v", err)
		return 0, fmt.Errorf("failed to parse price")
	}
	return price, nil
}

func convertedTonBitFinexResponse(respBody io.ReadCloser) (float64, error) {
	var prices []float64
	if err := json.NewDecoder(respBody).Decode(&prices); err != nil {
		log.Errorf("[convertedTonBitFinexResponse] failed to decode response: %v", err)
		return 0, err
	}
	if len(prices) == 0 {
		log.Errorf("[convertedTonBitFinexResponse] empty data")
		return 0, fmt.Errorf("empty data")
	}
	if len(prices) >= 6 { // last market price
		return prices[6], nil
	}
	return prices[0], nil
}

func convertedTonKuCoinResponse(respBody io.ReadCloser) (float64, error) {
	var data struct {
		Success bool `json:"success"`
		Data    []struct {
			LastTradedPrice string `json:"lastTradedPrice"`
		} `json:"data"`
	}
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {
		log.Errorf("[convertedTonKuCoinResponse] failed to decode response: %v", err)
		return 0, err
	}
	if !data.Success {
		log.Errorf("[convertedTonKuCoinResponse] unsuccessful response")
		return 0, fmt.Errorf("unsuccessful response")
	}
	if len(data.Data) == 0 {
		log.Errorf("[convertedTonKuCoinResponse] empty data")
		return 0, fmt.Errorf("empty data")
	}
	price, err := strconv.ParseFloat(data.Data[0].LastTradedPrice, 64)
	if err != nil {
		log.Errorf("[convertedTonKuCoinResponse] failed to parse price: %v", err)
		return 0, fmt.Errorf("failed to parse price")
	}
	return price, nil
}

func convertedTonOKXResponse(respBody io.ReadCloser) (float64, error) {
	var data struct {
		Code string `json:"code"`
		Data []struct {
			Last string `json:"last"`
		} `json:"data"`
	}
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {
		log.Errorf("[convertedTonOKXResponse] failed to decode response: %v", err)
		return 0, err
	}
	if data.Code != "0" {
		log.Errorf("[convertedTonOKXResponse] unsuccessful response")
		return 0, fmt.Errorf("unsuccessful response")
	}
	if len(data.Data) == 0 {
		log.Errorf("[convertedTonOKXResponse] empty data")
		return 0, fmt.Errorf("empty data")
	}
	price, err := strconv.ParseFloat(data.Data[0].Last, 64)
	if err != nil {
		log.Errorf("[convertedTonOKXResponse] failed to parse price: %v", err)
		return 0, fmt.Errorf("failed to parse price")
	}

	return price, nil
}

func convertedTonHuobiResponse(respBody io.ReadCloser) (float64, error) {
	var data struct {
		Status string `json:"status"`
		Tick   struct {
			Data []struct {
				Ts     int64   `json:"ts"`
				Amount float64 `json:"amount"`
				Price  float64 `json:"price"`
			} `json:"data"`
		} `json:"tick"`
	}
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {
		log.Errorf("[convertedTonHuobiResponse] failed to decode response: %v", err)
		return 0, err
	}
	if data.Status != "ok" {
		log.Errorf("[convertedTonHuobiResponse] unsuccessful response")
		return 0, fmt.Errorf("unsuccessful response")
	}
	if len(data.Tick.Data) == 0 {
		log.Errorf("[convertedTonHuobiResponse] empty data")
		return 0, fmt.Errorf("empty data")
	}

	return data.Tick.Data[0].Price, nil
}

func getFiatPrices() map[string]float64 {
	markets := []Market{
		{
			Name:               coinbase,
			ApiURL:             "https://api.coinbase.com/v2/exchange-rates?currency=USD",
			FiatPriceConverter: convertedCoinBaseFiatPricesResponse,
		},
		{
			Name:               exchangerate,
			ApiURL:             "https://api.exchangerate.host/latest?base=USD",
			FiatPriceConverter: convertedExchangerateFiatPricesResponse,
		},
	}
	prices := make(map[string]float64)
	for _, market := range markets {
		respBody, err := sendRequest(market.ApiURL)
		if err != nil {
			errorsCounter.WithLabelValues(market.Name).Inc()
			continue
		}
		converted := market.FiatPriceConverter(respBody)
		for currency, rate := range converted {
			if _, ok := prices[currency]; !ok {
				prices[currency] = rate
			}
		}
	}
	return prices
}

func convertedExchangerateFiatPricesResponse(respBody io.ReadCloser) map[string]float64 {
	var data struct {
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {
		log.Errorf("[convertedExchangerateFiatPricesResponse] failed to decode response: %v", err)
		return map[string]float64{}
	}
	prices := make(map[string]float64)
	for currency, rate := range data.Rates {
		prices[currency] = rate
	}
	return prices
}

func convertedCoinBaseFiatPricesResponse(respBody io.ReadCloser) map[string]float64 {
	var data struct {
		Data struct {
			Rates map[string]string `json:"rates"`
		} `json:"data"`
	}
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {
		log.Errorf("[convertedCoinBaseFiatPricesResponse] failed to decode response: %v", err)
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

func getPools(tonPrice float64, storage storage) map[ton.AccountID]float64 {
	markets := []Market{
		{
			Name:                  dedust,
			ApiURL:                "https://api.dedust.io/v2/pools",
			PoolResponseConverter: convertedDedustPoolResponse,
		},
		{
			Name:                  stonfi,
			ApiURL:                "https://api.ston.fi/v1/assets",
			PoolResponseConverter: convertedStonFiPoolResponse,
		},
		{
			Name:                  megaton,
			ApiURL:                "https://megaton.fi/api/token/infoList",
			PoolResponseConverter: convertedMegatonPoolResponse,
		},
	}
	pools := make(map[ton.AccountID]float64)
	for _, market := range markets {
		respBody, err := sendRequest(market.ApiURL)
		if err != nil {
			errorsCounter.WithLabelValues(market.Name).Inc()
			continue
		}
		convertedPools := market.PoolResponseConverter(storage, tonPrice, respBody)
		for currency, rate := range convertedPools {
			if _, ok := pools[currency]; !ok {
				pools[currency] = rate
			}
		}
	}
	return pools
}

func convertedMegatonPoolResponse(storage storage, tonPrice float64, respBody io.ReadCloser) map[ton.AccountID]float64 {
	var data []struct {
		Address string  `json:"address"`
		Price   float64 `json:"price"`
	}
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {
		log.Errorf("[convertedMegatonPoolResponse] failed to decode response: %v", err)
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

func convertedStonFiPoolResponse(storage storage, tonPrice float64, respBody io.ReadCloser) map[ton.AccountID]float64 {
	var data struct {
		AssetList []struct {
			ContractAddress string  `json:"contract_address"`
			DexUsdPrice     *string `json:"dex_usd_price"`
		} `json:"asset_list"`
	}
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {
		log.Errorf("[convertedStonFiPoolResponse] failed to decode response: %v", err)
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

func convertedDedustPoolResponse(storage storage, tonPrice float64, respBody io.ReadCloser) map[ton.AccountID]float64 {
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
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {
		log.Errorf("[convertedDedustPoolResponse] failed to decode response: %v", err)
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
