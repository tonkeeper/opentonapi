package rates

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
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
	coinbase     string = "Coinbase"
	exchangerate string = "Exchangerate"
)

const minReserve = float64(100 * ton.OneTON)

type Market struct {
	ID                    int64
	Name                  string
	UsdPrice              float64
	URL                   string
	TonPriceConverter     func(closer io.ReadCloser) (float64, error)
	FiatPriceConverter    func(closer io.ReadCloser) map[string]float64
	PoolResponseConverter func(pools map[ton.AccountID]float64, respBody io.ReadCloser) (map[ton.AccountID]float64, error)
	DateUpdate            time.Time
}

func (m *Mock) GetCurrentMarketsTonPrice() ([]Market, error) {
	now := time.Now()
	markets := []Market{
		{
			ID:                1,
			Name:              bitfinex,
			URL:               "https://api-pub.bitfinex.com/v2/ticker/tTONUSD",
			TonPriceConverter: convertedTonBitFinexResponse,
			DateUpdate:        now,
		},
		{
			ID:                2,
			Name:              gateio,
			URL:               "https://api.gateio.ws/api/v4/spot/tickers?currency_pair=TON_USDT",
			TonPriceConverter: convertedTonGateIOResponse,
			DateUpdate:        now,
		},
		{
			ID:                3,
			Name:              bybit,
			URL:               "https://api.bybit.com/derivatives/v3/public/tickers?symbol=TONUSDT",
			TonPriceConverter: convertedTonBybitResponse,
			DateUpdate:        now,
		},
		{
			ID:                4,
			Name:              kucoin,
			URL:               "https://www.kucoin.com/_api/trade-front/market/getSymbolTick?symbols=TON-USDT",
			TonPriceConverter: convertedTonKuCoinResponse,
			DateUpdate:        now,
		},
		{
			ID:                5,
			Name:              okx,
			URL:               "https://www.okx.com/api/v5/market/ticker?instId=TON-USDT",
			TonPriceConverter: convertedTonOKXResponse,
			DateUpdate:        now,
		},
		{
			ID:                6,
			Name:              huobi,
			URL:               "https://api.huobi.pro/market/trade?symbol=tonusdt",
			TonPriceConverter: convertedTonHuobiResponse,
			DateUpdate:        now,
		},
	}
	for idx, market := range markets {
		respBody, err := sendRequest(market.URL, "")
		if err != nil {
			zap.Error(fmt.Errorf("[GetCurrentMarketsTonPrice] failed to send request: %v", err))
			continue
		}
		market.UsdPrice, err = market.TonPriceConverter(respBody)
		if err != nil {
			zap.Error(fmt.Errorf("[GetCurrentMarketsTonPrice] failed to convert ton price: %v", err))
			continue
		}
		if market.UsdPrice == 0 {
			continue
		}
		markets[idx] = market
	}
	sort.Slice(markets, func(i, j int) bool {
		return markets[i].ID > markets[j].ID
	})
	return markets, nil
}

func sendRequest(url, token string) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", token))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		var errRespBody string
		if respBody, err := io.ReadAll(resp.Body); err == nil {
			errRespBody = string(respBody)
		}
		return nil, fmt.Errorf("bad status code: %v %v %v", resp.StatusCode, url, errRespBody)
	}
	return resp.Body, nil
}

func convertedTonGateIOResponse(respBody io.ReadCloser) (float64, error) {
	defer respBody.Close()
	var data []struct {
		Last string `json:"last"`
	}
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {
		return 0, fmt.Errorf("[convertedTonGateIOResponse] failed to decode response: %v", err)
	}
	if len(data) == 0 {
		return 0, fmt.Errorf("[convertedTonGateIOResponse] empty data")
	}
	price, err := strconv.ParseFloat(data[0].Last, 64)
	if err != nil {
		return 0, fmt.Errorf("[convertedTonGateIOResponse] failed to parse price: %v", err)
	}
	return price, nil
}

func convertedTonBybitResponse(respBody io.ReadCloser) (float64, error) {
	defer respBody.Close()
	var data struct {
		RetMsg string `json:"retMsg"`
		Result struct {
			List []struct {
				LastPrice string `json:"lastPrice"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {
		return 0, fmt.Errorf("[convertedTonBybitResponse] failed to decode response: %v", err)
	}
	if data.RetMsg != "OK" {
		return 0, fmt.Errorf("[convertedTonBybitResponse] unsuccessful response")
	}
	if len(data.Result.List) == 0 {
		return 0, fmt.Errorf("[convertedTonBybitResponse] empty data")
	}
	price, err := strconv.ParseFloat(data.Result.List[0].LastPrice, 64)
	if err != nil {
		return 0, fmt.Errorf("[convertedTonBybitResponse] failed to parse price: %v", err)
	}
	return price, nil
}

func convertedTonBitFinexResponse(respBody io.ReadCloser) (float64, error) {
	defer respBody.Close()
	var prices []float64
	if err := json.NewDecoder(respBody).Decode(&prices); err != nil {
		return 0, fmt.Errorf("[convertedTonBitFinexResponse] failed to decode response: %v", err)
	}
	if len(prices) == 0 {

		return 0, fmt.Errorf("[convertedTonBitFinexResponse] empty data")
	}
	if len(prices) >= 6 { // last market price
		return prices[6], nil
	}
	return prices[0], nil
}

func convertedTonKuCoinResponse(respBody io.ReadCloser) (float64, error) {
	defer respBody.Close()
	var data struct {
		Success bool `json:"success"`
		Data    []struct {
			LastTradedPrice string `json:"lastTradedPrice"`
		} `json:"data"`
	}
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {

		return 0, fmt.Errorf("[convertedTonKuCoinResponse] failed to decode response: %v", err)
	}
	if !data.Success {
		return 0, fmt.Errorf("[convertedTonKuCoinResponse] unsuccessful response")
	}
	if len(data.Data) == 0 {
		return 0, fmt.Errorf("[convertedTonKuCoinResponse] empty data")
	}
	price, err := strconv.ParseFloat(data.Data[0].LastTradedPrice, 64)
	if err != nil {
		return 0, fmt.Errorf("[convertedTonKuCoinResponse] failed to parse price: %v", err)
	}
	return price, nil
}

func convertedTonOKXResponse(respBody io.ReadCloser) (float64, error) {
	defer respBody.Close()
	var data struct {
		Code string `json:"code"`
		Data []struct {
			Last string `json:"last"`
		} `json:"data"`
	}
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {

		return 0, fmt.Errorf("[convertedTonOKXResponse] failed to decode response: %v", err)
	}
	if data.Code != "0" {
		return 0, fmt.Errorf("[convertedTonOKXResponse] unsuccessful response")
	}
	if len(data.Data) == 0 {
		return 0, fmt.Errorf("[convertedTonOKXResponse] empty data")
	}
	price, err := strconv.ParseFloat(data.Data[0].Last, 64)
	if err != nil {
		zap.Error(fmt.Errorf("[convertedTonOKXResponse] failed to parse price: %v", err))
		return 0, fmt.Errorf("failed to parse price")
	}

	return price, nil
}

func convertedTonHuobiResponse(respBody io.ReadCloser) (float64, error) {
	defer respBody.Close()
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
		return 0, fmt.Errorf("[convertedTonHuobiResponse] failed to decode response: %v", err)
	}
	if data.Status != "ok" {
		return 0, fmt.Errorf("[convertedTonHuobiResponse] unsuccessful response")
	}
	if len(data.Tick.Data) == 0 {
		return 0, fmt.Errorf("[convertedTonHuobiResponse] empty data")
	}

	return data.Tick.Data[0].Price, nil
}

func getFiatPrices(tonPrice float64) map[string]float64 {
	markets := []Market{
		{
			Name:               coinbase,
			URL:                "https://api.coinbase.com/v2/exchange-rates?currency=USD",
			FiatPriceConverter: convertedCoinBaseFiatPricesResponse,
		},
		{
			Name:               exchangerate,
			URL:                "https://api.exchangerate.host/latest?base=USD",
			FiatPriceConverter: convertedExchangerateFiatPricesResponse,
		},
	}
	prices := make(map[string]float64)
	for _, market := range markets {
		respBody, err := sendRequest(market.URL, "")
		if err != nil {
			zap.Error(fmt.Errorf("[getFiatPrices] failed to send request: %v", err))
			errorsCounter.WithLabelValues(market.Name).Inc()
			continue
		}
		converted := market.FiatPriceConverter(respBody)
		for currency, rate := range converted {
			if _, ok := prices[currency]; !ok && rate != 0 {
				prices[currency] = 1 / (rate * tonPrice)
			}
		}
	}
	return prices
}

func convertedExchangerateFiatPricesResponse(respBody io.ReadCloser) map[string]float64 {
	defer respBody.Close()
	var data struct {
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {
		//todo: return err
		return map[string]float64{}
	}
	prices := make(map[string]float64)
	for currency, rate := range data.Rates {
		prices[currency] = rate
	}
	return prices
}

func convertedCoinBaseFiatPricesResponse(respBody io.ReadCloser) map[string]float64 {
	defer respBody.Close()
	var data struct {
		Data struct {
			Rates map[string]string `json:"rates"`
		} `json:"data"`
	}
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {
		//todo: return err
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

func (m *Mock) getPools() map[ton.AccountID]float64 {
	markets := []Market{
		{
			Name:                  dedust,
			URL:                   m.DedustResultUrl,
			PoolResponseConverter: convertedDedustPoolResponse,
		},
		{
			Name:                  stonfi,
			URL:                   m.StonFiResultUrl,
			PoolResponseConverter: convertedStonFiPoolResponse,
		},
	}
	pools := make(map[ton.AccountID]float64)
	for attempt := 0; attempt < 2; attempt++ {
		for _, market := range markets {
			respBody, err := sendRequest(market.URL, "")
			if err != nil {
				zap.Error(fmt.Errorf("[getPools] failed to send request: %v", err))
				errorsCounter.WithLabelValues(market.Name).Inc()
				continue
			}
			updatedPools, err := market.PoolResponseConverter(pools, respBody)
			if err != nil {
				continue
			}
			for currency, rate := range updatedPools {
				if _, ok := pools[currency]; !ok {
					pools[currency] = rate
				}
			}
		}
	}
	return pools
}

func convertedStonFiPoolResponse(pools map[ton.AccountID]float64, respBody io.ReadCloser) (map[ton.AccountID]float64, error) {
	defer respBody.Close()
	pools[references.PTon] = 1 // pTon = TON
	reader := csv.NewReader(respBody)
	records, err := reader.ReadAll()
	if err != nil {
		return map[ton.AccountID]float64{}, err
	}
	calculate := func(record []string) (ton.AccountID, float64, error) {
		firstAsset, err := ton.ParseAccountID(record[0])
		if err != nil {
			return ton.AccountID{}, 0, err
		}
		secondAsset, err := ton.ParseAccountID(record[1])
		if err != nil {
			return ton.AccountID{}, 0, err
		}
		firstReserve, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			return ton.AccountID{}, 0, err
		}
		secondReserve, err := strconv.ParseFloat(record[3], 64)
		if err != nil {
			return ton.AccountID{}, 0, err
		}
		firstMeta := make(map[string]string)
		if err = json.Unmarshal([]byte(record[4]), &firstMeta); err != nil {
			return ton.AccountID{}, 0, err
		}
		firstDecimals, err := strconv.Atoi(firstMeta["decimals"])
		if err != nil {
			return ton.AccountID{}, 0, err
		}
		secondMeta := make(map[string]string)
		if err = json.Unmarshal([]byte(record[5]), &secondMeta); err != nil {
			return ton.AccountID{}, 0, err
		}
		secondDecimals, err := strconv.Atoi(secondMeta["decimals"])
		if err != nil {
			return ton.AccountID{}, 0, err
		}
		calculatedAccount, price := calculatePoolPrice(firstAsset, secondAsset, firstReserve, secondReserve, firstDecimals, secondDecimals, pools, false)

		return calculatedAccount, price, nil
	}
	for idx, record := range records {
		if idx == 0 || len(record) < 6 { // skip headers
			continue
		}
		accountID, price, err := calculate(record)
		if price == 0 || err != nil {
			continue
		}
		if _, ok := pools[accountID]; !ok {
			pools[accountID] = price
		}
	}
	return pools, nil
}

func convertedDedustPoolResponse(pools map[ton.AccountID]float64, respBody io.ReadCloser) (map[ton.AccountID]float64, error) {
	defer respBody.Close()
	reader := csv.NewReader(respBody)
	records, err := reader.ReadAll()
	if err != nil {
		return map[ton.AccountID]float64{}, err
	}
	var zeroAddress = ton.MustParseAccountID("UQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAJKZ")
	pools[zeroAddress] = 1
	parseDecimals := func(meta string) (int, error) {
		const defaultDecimals = 9
		if meta == "NULL" {
			return defaultDecimals, nil
		}
		converted := make(map[string]string)
		if err = json.Unmarshal([]byte(meta), &converted); err != nil {
			return 0, err
		}
		decimals, err := strconv.Atoi(converted["decimals"])
		if err != nil {
			return 0, err
		}
		return decimals, nil
	}
	calculate := func(record []string) (ton.AccountID, float64, error) {
		var firstAsset, secondAsset ton.AccountID
		switch {
		case record[0] == "NULL" && record[2] == "true":
			firstAsset = zeroAddress
			secondAsset, err = ton.ParseAccountID(record[1])
		case record[1] == "NULL" && record[3] != "true":
			firstAsset, err = ton.ParseAccountID(record[0])
			secondAsset = zeroAddress
		default:
			firstAsset, err = ton.ParseAccountID(record[0])
			if err == nil {
				secondAsset, err = ton.ParseAccountID(record[1])
			}
		}
		if err != nil {
			return ton.AccountID{}, 0, err
		}
		firstReserve, err := strconv.ParseFloat(record[4], 64)
		if err != nil {
			return ton.AccountID{}, 0, err
		}
		secondReserve, err := strconv.ParseFloat(record[5], 64)
		if err != nil {
			return ton.AccountID{}, 0, err
		}
		firstDecimals, err := parseDecimals(record[6])
		if err != nil {
			return ton.AccountID{}, 0, err
		}
		secondDecimals, err := parseDecimals(record[7])
		if err != nil {
			return ton.AccountID{}, 0, err
		}
		var isStable bool
		if record[8] == "true" {
			isStable = true
		}
		calculatedAccount, price := calculatePoolPrice(firstAsset, secondAsset, firstReserve, secondReserve, firstDecimals, secondDecimals, pools, isStable)

		return calculatedAccount, price, nil
	}

	for idx, record := range records {
		if idx == 0 || len(record) < 8 { // skip headers
			continue
		}
		accountID, price, err := calculate(record)
		if price == 0 || err != nil {
			continue
		}
		if _, ok := pools[accountID]; !ok {
			pools[accountID] = price
		}
	}

	return pools, nil
}

func calculatePoolPrice(firstAsset, secondAsset ton.AccountID, firstReserve, secondReserve float64, firstDecimals, secondDecimals int, pools map[ton.AccountID]float64, isStable bool) (ton.AccountID, float64) {
	priceFirst, okFirst := pools[firstAsset]
	priceSecond, okSecond := pools[secondAsset]
	if (okFirst && okSecond) || (!okFirst && !okSecond) {
		return ton.AccountID{}, 0
	}
	var calculatedAccount ton.AccountID
	var decimals int
	if okFirst { // knowing the first account's price, we seek the second account's price
		firstReserve *= priceFirst
		if firstReserve < minReserve {
			return ton.AccountID{}, 0
		}
		calculatedAccount = secondAsset
		decimals = secondDecimals
	}
	if okSecond { // knowing the second account's price, we seek the first account's price
		secondReserve *= priceSecond
		if secondReserve < minReserve {
			return ton.AccountID{}, 0
		}
		calculatedAccount = firstAsset
		decimals = firstDecimals
		firstReserve, secondReserve = secondReserve, firstReserve
	}
	if decimals == 0 {
		return ton.AccountID{}, 0
	}
	var price float64
	if isStable {
		x := secondReserve / math.Pow(10, float64(decimals))
		y := firstReserve / math.Pow(10, 9)
		price = (3*x*x*y + y*y*y) / (x*x*x + 3*y*y*x)
	} else {
		price = (firstReserve / secondReserve) * math.Pow(10, float64(decimals)-9)
	}
	return calculatedAccount, price
}
