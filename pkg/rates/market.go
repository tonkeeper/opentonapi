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

// List of services used to calculate various prices
const (
	bitfinex string = "Bitfinex"
	gateio   string = "Gate.io"
	bybit    string = "Bybit"
	kucoin   string = "KuCoin"
	okx      string = "OKX"
	huobi    string = "Huobi"
	dedust   string = "DeDust"
	stonfi   string = "STON.fi"
	coinbase string = "Coinbase"
)

// minReserve specifies the minimum jetton reserves (equivalent to TON) for which prices can be determined
const minReserve = float64(100 * ton.OneTON)

// minHoldersCount minimum number of holders threshold for jettons
const minHoldersCount = 100

// defaultDecimals sets the default number of decimals to 9, similar to TON
const defaultDecimals = 9

// Asset represents an asset used in jetton price calculations within pools
type Asset struct {
	Account      ton.AccountID
	Decimals     int
	Reserve      float64
	HoldersCount int
}

// DeDustAssets represents a collection of assets from the DeDust platform, including their stability status
type DeDustAssets struct {
	Assets   []Asset
	IsStable bool
}

type Market struct {
	ID       int64
	Name     string // Name of the service used for price calculation
	UsdPrice float64
	URL      string
	// Converter for calculating the TON to USD price
	TonPriceConverter func(closer io.ReadCloser) (float64, error)
	// Converter for calculating fiat prices
	FiatPriceConverter func(closer io.ReadCloser) (map[string]float64, error)
	// Converter for calculating jetton prices within pools
	PoolResponseConverter func(pools map[ton.AccountID]float64, closer io.ReadCloser) (map[ton.AccountID]float64, error)
	DateUpdate            time.Time
}

// GetCurrentMarketsTonPrice shows the TON to USD price on different markets
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
			errorsCounter.WithLabelValues(market.Name).Inc()
			continue
		}
		market.UsdPrice, err = market.TonPriceConverter(respBody)
		if err != nil {
			zap.Error(fmt.Errorf("[GetCurrentMarketsTonPrice] failed to convert response: %v", err))
			errorsCounter.WithLabelValues(market.Name).Inc()
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
	if len(prices) >= 6 { // Price of the last trade
		return prices[6], nil
	}
	// Price of last highest bid
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

// getFiatPrices shows the exchange rates of various fiats to USD on different markets
func getFiatPrices(tonPrice float64) map[string]float64 {
	markets := []Market{
		{
			Name:               coinbase,
			URL:                "https://api.coinbase.com/v2/exchange-rates?currency=USD",
			FiatPriceConverter: convertedCoinBaseFiatPricesResponse,
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
		converted, err := market.FiatPriceConverter(respBody)
		if err != nil {
			zap.Error(fmt.Errorf("[getFiatPrices] failed to convert response: %v", err))
			errorsCounter.WithLabelValues(market.Name).Inc()
			continue
		}
		for currency, rate := range converted {
			if _, ok := prices[currency]; !ok && rate != 0 {
				prices[currency] = 1 / (rate * tonPrice)
			}
		}
	}
	return prices
}

func convertedCoinBaseFiatPricesResponse(respBody io.ReadCloser) (map[string]float64, error) {
	defer respBody.Close()
	var data struct {
		Data struct {
			Rates map[string]string `json:"rates"`
		} `json:"data"`
	}
	if err := json.NewDecoder(respBody).Decode(&data); err != nil {
		return map[string]float64{}, fmt.Errorf("[convertedCoinBaseFiatPricesResponse] failed to decode response: %v", err)
	}
	prices := make(map[string]float64)
	for currency, rate := range data.Data.Rates {
		if rateConverted, err := strconv.ParseFloat(rate, 64); err == nil {
			prices[currency] = rateConverted
		}
	}
	return prices, nil
}

// getPools calculates the price of jettons relative to TON based on liquidity pools
func (m *Mock) getPools() map[ton.AccountID]float64 {
	markets := []Market{
		{
			Name:                  dedust,
			URL:                   m.DedustResultUrl,
			PoolResponseConverter: convertedDeDustPoolResponse,
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
				zap.Error(fmt.Errorf("[getPools] failed to convert response: %v", err))
				errorsCounter.WithLabelValues(market.Name).Inc()
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
	parseAssets := func(record []string) (Asset, Asset, error) {
		var firstAsset, secondAsset Asset
		firstAsset.Account, err = ton.ParseAccountID(record[0])
		if err != nil {
			return Asset{}, Asset{}, err
		}
		secondAsset.Account, err = ton.ParseAccountID(record[1])
		if err != nil {
			return Asset{}, Asset{}, err
		}
		firstAsset.Reserve, err = strconv.ParseFloat(record[2], 64)
		if err != nil {
			return Asset{}, Asset{}, err
		}
		secondAsset.Reserve, err = strconv.ParseFloat(record[3], 64)
		if err != nil {
			return Asset{}, Asset{}, err
		}
		firstMeta := make(map[string]any)
		if err = json.Unmarshal([]byte(record[4]), &firstMeta); err != nil {
			return Asset{}, Asset{}, err
		}
		value, ok := firstMeta["decimals"]
		if !ok {
			value = fmt.Sprintf("%d", defaultDecimals)
		}
		firstAsset.Decimals, err = strconv.Atoi(value.(string))
		if err != nil {
			return Asset{}, Asset{}, err
		}
		secondMeta := make(map[string]any)
		if err = json.Unmarshal([]byte(record[5]), &secondMeta); err != nil {
			return Asset{}, Asset{}, err
		}
		value, ok = secondMeta["decimals"]
		if !ok {
			value = fmt.Sprintf("%d", defaultDecimals)
		}
		secondAsset.Decimals, err = strconv.Atoi(value.(string))
		if err != nil {
			return Asset{}, Asset{}, err
		}
		firstAsset.HoldersCount, err = strconv.Atoi(record[6])
		if err != nil {
			return Asset{}, Asset{}, err
		}
		secondAsset.HoldersCount, err = strconv.Atoi(record[7])
		if err != nil {
			return Asset{}, Asset{}, err
		}
		return firstAsset, secondAsset, nil
	}

	actualAssets := make(map[ton.AccountID][]Asset)
	// Update the assets with the largest reserves
	updateActualAssets := func(mainAsset Asset, firstAsset, secondAsset Asset) {
		assets, ok := actualAssets[mainAsset.Account]
		if !ok {
			actualAssets[mainAsset.Account] = []Asset{firstAsset, secondAsset}
			return
		}
		for idx, asset := range assets {
			if asset.Account == mainAsset.Account && asset.Reserve < mainAsset.Reserve {
				if idx == 0 {
					actualAssets[mainAsset.Account] = []Asset{firstAsset, secondAsset}
				} else {
					actualAssets[mainAsset.Account] = []Asset{secondAsset, firstAsset}
				}
			}
		}
	}
	for idx, record := range records {
		if idx == 0 || len(record) < 8 { // Skip headers
			continue
		}
		firstAsset, secondAsset, err := parseAssets(record)
		if err != nil {
			zap.Error(fmt.Errorf("[convertedStonFiPoolResponse] failed to parse assets: %v", err))
			continue
		}
		if firstAsset.Reserve == 0 || secondAsset.Reserve == 0 {
			continue
		}
		// PTon is the primary token on StonFi, but it has only 50 holders.
		// To avoid missing tokens, we check for pTON
		if (firstAsset.Account != references.PTon && firstAsset.HoldersCount < minHoldersCount) || (secondAsset.Account != references.PTon && secondAsset.HoldersCount < minHoldersCount) {
			continue
		}
		updateActualAssets(firstAsset, firstAsset, secondAsset)
		updateActualAssets(secondAsset, firstAsset, secondAsset)
	}
	for _, assets := range actualAssets {
		accountID, price := calculatePoolPrice(assets[0], assets[1], pools, false)
		if price == 0 {
			continue
		}
		if _, ok := pools[accountID]; !ok {
			pools[accountID] = price
		}
	}
	return pools, nil
}

func convertedDeDustPoolResponse(pools map[ton.AccountID]float64, respBody io.ReadCloser) (map[ton.AccountID]float64, error) {
	defer respBody.Close()
	reader := csv.NewReader(respBody)
	records, err := reader.ReadAll()
	if err != nil {
		return map[ton.AccountID]float64{}, err
	}
	var zeroAddress = ton.MustParseAccountID("UQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAJKZ")
	pools[zeroAddress] = 1
	parseDecimals := func(meta string) (int, error) {
		if meta == "NULL" {
			return defaultDecimals, nil
		}
		converted := make(map[string]any)
		if err = json.Unmarshal([]byte(meta), &converted); err != nil {
			return 0, err
		}
		value, ok := converted["decimals"]
		if !ok {
			value = "9"
		}
		decimals, err := strconv.Atoi(value.(string))
		if err != nil {
			return 0, err
		}
		return decimals, nil
	}
	parseAssets := func(record []string) (DeDustAssets, error) {
		var firstAsset, secondAsset Asset
		switch {
		// If the column first_asset has no address and the column first_asset_native contains true,
		// then we consider this token as a pool to TON
		case record[0] == "NULL" && record[2] == "true":
			firstAsset = Asset{Account: zeroAddress}
			secondAccountID, err := ton.ParseAccountID(record[1])
			if err != nil {
				return DeDustAssets{}, err
			}
			secondAsset = Asset{Account: secondAccountID}
		// If the column second_asset has no address and the column second_asset_native contains true,
		// then we consider this token as a pool to TON
		case record[1] == "NULL" && record[3] != "true":
			firstAccountID, err := ton.ParseAccountID(record[0])
			if err != nil {
				return DeDustAssets{}, err
			}
			firstAsset = Asset{Account: firstAccountID}
			secondAsset = Asset{Account: zeroAddress}
		default:
			// By default, we assume that the two assets are not paired with TON.
			// This could be a pair like a jetton to USDT or to other jettons
			firstAccountID, err := ton.ParseAccountID(record[0])
			if err != nil {
				return DeDustAssets{}, err
			}
			firstAsset = Asset{Account: firstAccountID}
			secondAccountID, err := ton.ParseAccountID(record[1])
			if err != nil {
				return DeDustAssets{}, err
			}
			secondAsset = Asset{Account: secondAccountID}
		}
		firstAsset.Reserve, err = strconv.ParseFloat(record[4], 64)
		if err != nil {
			return DeDustAssets{}, err
		}
		secondAsset.Reserve, err = strconv.ParseFloat(record[5], 64)
		if err != nil {
			return DeDustAssets{}, err
		}
		firstAsset.Decimals, err = parseDecimals(record[6])
		if err != nil {
			return DeDustAssets{}, err
		}
		secondAsset.Decimals, err = parseDecimals(record[7])
		if err != nil {
			return DeDustAssets{}, err
		}
		var isStable bool
		if record[8] == "true" {
			isStable = true
		}
		firstAsset.HoldersCount, err = strconv.Atoi(record[9])
		if err != nil {
			return DeDustAssets{}, err
		}
		secondAsset.HoldersCount, err = strconv.Atoi(record[10])
		if err != nil {
			return DeDustAssets{}, err
		}
		return DeDustAssets{Assets: []Asset{firstAsset, secondAsset}, IsStable: isStable}, nil
	}
	actualAssets := make(map[ton.AccountID]DeDustAssets)
	// Update the assets with the largest reserves
	updateActualAssets := func(mainAsset Asset, deDustAssets DeDustAssets) {
		firstAsset, secondAsset := deDustAssets.Assets[0], deDustAssets.Assets[1]
		assets, ok := actualAssets[mainAsset.Account]
		if !ok {
			actualAssets[mainAsset.Account] = DeDustAssets{Assets: []Asset{firstAsset, secondAsset}, IsStable: deDustAssets.IsStable}
			return
		}
		for idx, asset := range assets.Assets {
			if asset.Account == mainAsset.Account && asset.Reserve < mainAsset.Reserve {
				if idx == 0 {
					actualAssets[mainAsset.Account] = DeDustAssets{Assets: []Asset{firstAsset, secondAsset}, IsStable: deDustAssets.IsStable}
				} else {
					actualAssets[mainAsset.Account] = DeDustAssets{Assets: []Asset{secondAsset, firstAsset}, IsStable: deDustAssets.IsStable}
				}
			}
		}
	}
	for idx, record := range records {
		if idx == 0 || len(record) < 10 { // Skip headers
			continue
		}
		assets, err := parseAssets(record)
		if err != nil {
			zap.Error(fmt.Errorf("[convertedDedustPoolResponse] failed to parse assets: %v", err))
			continue
		}
		firstAsset, secondAsset := assets.Assets[0], assets.Assets[1]
		if firstAsset.Reserve == 0 || secondAsset.Reserve == 0 {
			continue
		}
		updateActualAssets(firstAsset, assets)
		updateActualAssets(secondAsset, assets)
	}
	for _, pool := range actualAssets {
		accountID, price := calculatePoolPrice(pool.Assets[0], pool.Assets[1], pools, pool.IsStable)
		if price == 0 {
			continue
		}
		if _, ok := pools[accountID]; !ok {
			pools[accountID] = price
		}
	}

	return pools, nil
}

func calculatePoolPrice(firstAsset, secondAsset Asset, pools map[ton.AccountID]float64, isStable bool) (ton.AccountID, float64) {
	priceFirst, okFirst := pools[firstAsset.Account]
	priceSecond, okSecond := pools[secondAsset.Account]
	if (okFirst && okSecond) || (!okFirst && !okSecond) {
		return ton.AccountID{}, 0
	}
	var calculatedAccount ton.AccountID // The account for which we will find the price
	var firstAssetDecimals, secondAssetDecimals int
	if okFirst { // Knowing the first asset's price, we determine the second asset's price
		// Converting reserve prices to TON
		var updatedFirstAssetReserve float64
		if firstAsset.Decimals != defaultDecimals {
			updatedFirstAssetReserve = firstAsset.Reserve * math.Pow(10, float64(secondAsset.Decimals)-float64(firstAsset.Decimals)) * priceFirst
		} else {
			firstAsset.Reserve *= priceFirst
			updatedFirstAssetReserve = firstAsset.Reserve
		}
		if updatedFirstAssetReserve < minReserve {
			return ton.AccountID{}, 0
		}
		if secondAsset.HoldersCount < minHoldersCount {
			return ton.AccountID{}, 0
		}
		calculatedAccount = secondAsset.Account
		firstAssetDecimals, secondAssetDecimals = firstAsset.Decimals, secondAsset.Decimals
	}
	if okSecond { // Knowing the second asset's price, we determine the first asset's price
		// Converting reserve prices to TON
		var updatedSecondAssetReserve float64
		if secondAsset.Decimals != defaultDecimals {
			updatedSecondAssetReserve = secondAsset.Reserve * math.Pow(10, float64(firstAsset.Decimals)-float64(secondAsset.Decimals)) * priceSecond
		} else {
			secondAsset.Reserve *= priceSecond
			updatedSecondAssetReserve = secondAsset.Reserve
		}
		if updatedSecondAssetReserve < minReserve {
			return ton.AccountID{}, 0
		}
		if firstAsset.HoldersCount < minHoldersCount {
			return ton.AccountID{}, 0
		}
		calculatedAccount = firstAsset.Account
		firstAsset, secondAsset = secondAsset, firstAsset
		firstAssetDecimals, secondAssetDecimals = firstAsset.Decimals, secondAsset.Decimals
	}
	if firstAssetDecimals == 0 || secondAssetDecimals == 0 {
		return ton.AccountID{}, 0
	}
	var price float64
	if isStable {
		x := secondAsset.Reserve / math.Pow(10, float64(secondAssetDecimals))
		y := firstAsset.Reserve / math.Pow(10, float64(firstAssetDecimals))
		price = (3*x*x*y + y*y*y) / (x*x*x + 3*y*y*x)
	} else {
		price = (firstAsset.Reserve / secondAsset.Reserve) * math.Pow(10, float64(secondAssetDecimals)-float64(firstAssetDecimals))
	}
	if okFirst && firstAsset.Decimals != defaultDecimals {
		price *= priceFirst
	}
	// Use firstAsset because after the revert, firstAsset equals secondAsset
	if okSecond && firstAsset.Decimals != defaultDecimals {
		price *= priceSecond
	}

	return calculatedAccount, price
}

// Note: You must close resp.Body in the handler function; here, it is closed ONLY in case of a bad status_code
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
		resp.Body.Close()
		return nil, fmt.Errorf("bad status code: %v %v %v", resp.StatusCode, url, errRespBody)
	}
	return resp.Body, nil
}
