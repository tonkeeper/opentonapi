package rates

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/ton"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slog"
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
	stonfiV1 string = "STON.fi v1"
	stonfiV2 string = "STON.fi v2"
	coinbase string = "Coinbase"
)

// defaultMinReserve specifies the minimum jetton reserves (equivalent to TON) for which prices can be determined
const defaultMinReserve = float64(100 * ton.OneTON)

// defaultMinHoldersCount minimum number of holders threshold for jettons
const defaultMinHoldersCount = 100

// defaultDecimals sets the default number of decimals to 9, similar to TON
const defaultDecimals = 9

// Asset represents an asset used in jetton price calculations within pools
type Asset struct {
	Account      ton.AccountID
	Decimals     int
	Reserve      float64
	HoldersCount int
}

// Assets represents a collection of assets in a pool
type Assets struct {
	Assets   []Asset
	IsStable bool
}

// LpAsset represents a liquidity provider asset that holds a collection of assets in a pool
type LpAsset struct {
	Account     ton.AccountID
	Decimals    int
	TotalSupply *big.Int // The total supply of the liquidity provider asset
	Assets      []Asset  // A slice of Asset included in the liquidity pool
}

type Market struct {
	ID       int64
	Name     string // Name of the service used for price calculation
	UsdPrice float64
	URL      string
	// Converter for calculating the TON to USD price
	TonPriceConverter func(respBody []byte) (float64, error)
	// Converter for calculating fiat prices
	FiatPriceConverter func(respBody []byte) (map[string]float64, error)
	// Converter for calculating jetton prices within pools
	PoolResponseConverter func(respBody []byte) ([]Assets, []LpAsset, error)
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
			URL:               "https://api.bybit.com/v5/market/tickers?category=spot&symbol=TONUSDT",
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
		headers := http.Header{"Content-Type": {"application/json"}}
		respBody, err := sendRequest(market.URL, "", headers)
		if err != nil {
			slog.Error("[GetCurrentMarketsTonPrice] failed to send request", slog.Any("error", err))
			errorsCounter.WithLabelValues(market.Name).Inc()
			continue
		}
		market.UsdPrice, err = market.TonPriceConverter(respBody)
		if err != nil {
			slog.Error("[GetCurrentMarketsTonPrice] failed to convert response", slog.Any("error", err))
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

func convertedTonGateIOResponse(respBody []byte) (float64, error) {
	var data []struct {
		Last string `json:"last"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
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

func convertedTonBybitResponse(respBody []byte) (float64, error) {
	var data struct {
		RetMsg string `json:"retMsg"`
		Result struct {
			List []struct {
				LastPrice string `json:"lastPrice"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
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

func convertedTonBitFinexResponse(respBody []byte) (float64, error) {
	var prices []float64
	if err := json.Unmarshal(respBody, &prices); err != nil {
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

func convertedTonKuCoinResponse(respBody []byte) (float64, error) {
	var data struct {
		Success bool `json:"success"`
		Data    []struct {
			LastTradedPrice string `json:"lastTradedPrice"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
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

func convertedTonOKXResponse(respBody []byte) (float64, error) {
	var data struct {
		Code string `json:"code"`
		Data []struct {
			Last string `json:"last"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
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
		slog.Error("[convertedTonOKXResponse] failed to parse price", slog.Any("error", err))
		return 0, fmt.Errorf("failed to parse price")
	}

	return price, nil
}

func convertedTonHuobiResponse(respBody []byte) (float64, error) {
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
	if err := json.Unmarshal(respBody, &data); err != nil {
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
		headers := http.Header{"Content-Type": {"application/json"}}
		respBody, err := sendRequest(market.URL, "", headers)
		if err != nil {
			slog.Error("[getFiatPrices] failed to send request", slog.Any("error", err))
			errorsCounter.WithLabelValues(market.Name).Inc()
			continue
		}
		converted, err := market.FiatPriceConverter(respBody)
		if err != nil {
			slog.Error("[getFiatPrices] failed to convert response", slog.Any("error", err))
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

func convertedCoinBaseFiatPricesResponse(respBody []byte) (map[string]float64, error) {
	var data struct {
		Data struct {
			Rates map[string]string `json:"rates"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
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

// sortAssetPairs sorts asset pairs by reserve for a given account.
func sortAssetPairs(assetPairs map[ton.AccountID][]Assets) map[ton.AccountID][]Assets {
	sortReserve := func(accountID ton.AccountID, item Assets) float64 {
		for _, asset := range item.Assets {
			if asset.Account == accountID {
				return asset.Reserve
			}
		}
		return 0.0
	}
	for account, pairs := range assetPairs {
		sort.Slice(pairs, func(i, j int) bool {
			return sortReserve(account, pairs[i]) > sortReserve(account, pairs[j])
		})
		assetPairs[account] = pairs
	}
	return assetPairs
}

// getJettonPricesFromDex calculates the price of jettons relative to TON based on liquidity pools
func (m *Mock) getJettonPricesFromDex(pools map[ton.AccountID]float64) map[ton.AccountID]float64 {
	// Define markets to fetch pool data from, each with a corresponding response converter
	markets := []Market{
		{
			Name:                  dedust,
			URL:                   m.DedustResultUrl,
			PoolResponseConverter: convertedDeDustPoolResponse,
		},
		{
			Name:                  stonfiV1,
			URL:                   m.StonV1FiResultUrl,
			PoolResponseConverter: convertedStonFiPoolResponse,
		},
		{
			Name:                  stonfiV2,
			URL:                   m.StonV2FiResultUrl,
			PoolResponseConverter: convertedStonFiPoolResponse,
		},
	}
	var actualAssets []Assets
	var actualLpAssets []LpAsset
	// Fetch and parse pool data from each market
	for _, market := range markets {
		headers := http.Header{"Accept": {"text/csv"}}
		respBody, err := sendRequest(market.URL, "", headers)
		if err != nil {
			slog.Error("[getJettonPricesFromDex] failed to send request", slog.Any("error", err), slog.String("url", market.URL))
			errorsCounter.WithLabelValues(market.Name).Inc()
			continue
		}
		assets, lpAssets, err := market.PoolResponseConverter(respBody)
		if err != nil {
			slog.Error("[getJettonPricesFromDex] failed to convert response", slog.Any("error", err))
			errorsCounter.WithLabelValues(market.Name).Inc()
			continue
		}
		actualAssets = append(actualAssets, assets...)
		actualLpAssets = append(actualLpAssets, lpAssets...)
	}
	// Map accounts to their participating pools
	assetPairs := make(map[ton.AccountID][]Assets)
	for _, assets := range actualAssets {
		firstAsset, secondAsset := assets.Assets[0], assets.Assets[1]
		assetPairs[firstAsset.Account] = append(assetPairs[firstAsset.Account], assets)
		assetPairs[secondAsset.Account] = append(assetPairs[secondAsset.Account], assets)
	}
	// Sort assets by reserve amount for each account in descending order
	assetPairs = sortAssetPairs(assetPairs)
	// Calculate and update prices for assets
	for attempt := 0; attempt < 3; attempt++ {
		for _, assets := range assetPairs {
			for _, asset := range assets {
				accountID, price := calculatePoolPrice(asset.Assets[0], asset.Assets[1], pools, asset.IsStable)
				if price == 0 {
					continue
				}
				if _, ok := pools[accountID]; !ok {
					pools[accountID] = price
					break
				}
			}
		}
	}
	// Calculate and update prices for LP assets
	for _, asset := range actualLpAssets {
		if _, ok := pools[asset.Account]; ok {
			continue
		}
		price := calculateLpAssetPrice(asset, pools)
		if price == 0 {
			continue
		}
		pools[asset.Account] = price
	}

	return pools
}

func convertedStonFiPoolResponse(respBody []byte) ([]Assets, []LpAsset, error) {
	reader := csv.NewReader(bytes.NewReader(respBody))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	parseAssets := func(record []string) (Assets, error) {
		var firstAsset, secondAsset Asset
		firstAsset.Account, err = ton.ParseAccountID(record[0])
		if err != nil {
			return Assets{}, err
		}
		secondAsset.Account, err = ton.ParseAccountID(record[1])
		if err != nil {
			return Assets{}, err
		}
		firstAsset.Reserve, err = strconv.ParseFloat(record[2], 64)
		if err != nil {
			return Assets{}, err
		}
		secondAsset.Reserve, err = strconv.ParseFloat(record[3], 64)
		if err != nil {
			return Assets{}, err
		}
		firstMeta := make(map[string]any)
		if record[4] != "NULL" {
			if err = json.Unmarshal([]byte(record[4]), &firstMeta); err != nil {
				return Assets{}, err
			}
		}
		value, ok := firstMeta["decimals"]
		if !ok || value != "NaN" {
			value = fmt.Sprintf("%d", defaultDecimals)
		}
		firstAsset.Decimals, err = strconv.Atoi(value.(string))
		if err != nil {
			return Assets{}, err
		}
		secondMeta := make(map[string]any)
		if record[5] != "NULL" {
			if err = json.Unmarshal([]byte(record[5]), &secondMeta); err != nil {
				return Assets{}, err
			}
		}
		value, ok = secondMeta["decimals"]
		if !ok || value != "NaN" {
			value = fmt.Sprintf("%d", defaultDecimals)
		}
		secondAsset.Decimals, err = strconv.Atoi(value.(string))
		if err != nil {
			return Assets{}, err
		}
		firstAsset.HoldersCount, err = strconv.Atoi(record[6])
		if err != nil {
			return Assets{}, err
		}
		secondAsset.HoldersCount, err = strconv.Atoi(record[7])
		if err != nil {
			return Assets{}, err
		}
		return Assets{Assets: []Asset{firstAsset, secondAsset}}, nil
	}
	parseLpAsset := func(record []string, firstAsset, secondAsset Asset) (LpAsset, error) {
		lpAsset, err := ton.ParseAccountID(record[8])
		if err != nil {
			return LpAsset{}, err
		}
		if record[9] == "0" {
			return LpAsset{}, fmt.Errorf("unknown total supply")
		}
		totalSupply, ok := new(big.Int).SetString(record[9], 10)
		if !ok {
			return LpAsset{}, fmt.Errorf("failed to parse total supply")
		}
		decimals, err := strconv.Atoi(record[10])
		if err != nil {
			return LpAsset{}, err
		}
		return LpAsset{
			Account:     lpAsset,
			Decimals:    decimals,
			TotalSupply: totalSupply,
			Assets:      []Asset{firstAsset, secondAsset},
		}, nil
	}
	var actualAssets []Assets
	actualLpAssets := make(map[ton.AccountID]LpAsset)
	for idx, record := range records {
		if idx == 0 || len(record) < 10 { // Skip headers
			continue
		}
		assets, err := parseAssets(record)
		if err != nil {
			slog.Error("failed to parse assets", slog.Any("error", err), slog.Any("assets", record))
			continue
		}
		firstAsset, secondAsset := assets.Assets[0], assets.Assets[1]
		if firstAsset.Reserve == 0 || secondAsset.Reserve == 0 {
			continue
		}
		// PTon is the primary token on StonFi, but it has only 50 holders.
		if firstAsset.Account == references.PTonV1 || firstAsset.Account == references.PTonV2 {
			firstAsset.HoldersCount = defaultMinHoldersCount
		}
		if secondAsset.Account == references.PTonV1 || secondAsset.Account == references.PTonV2 {
			secondAsset.HoldersCount = defaultMinHoldersCount
		}
		actualAssets = append(actualAssets, assets)
		lpAsset, err := parseLpAsset(record, firstAsset, secondAsset)
		if err != nil {
			continue
		}
		actualLpAssets[lpAsset.Account] = lpAsset
	}
	return actualAssets, maps.Values(actualLpAssets), nil
}

func convertedDeDustPoolResponse(respBody []byte) ([]Assets, []LpAsset, error) {
	reader := csv.NewReader(bytes.NewReader(respBody))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	parseDecimals := func(meta string) (int, error) {
		if meta == "NULL" {
			return defaultDecimals, nil
		}
		converted := make(map[string]any)
		if err = json.Unmarshal([]byte(meta), &converted); err != nil {
			return 0, err
		}
		value, ok := converted["decimals"]
		if !ok || value == "NaN" {
			value = fmt.Sprintf("%d", defaultDecimals)
		}
		decimals, err := strconv.Atoi(value.(string))
		if err != nil {
			return 0, err
		}
		return decimals, nil
	}
	parseAssets := func(record []string) (Assets, error) {
		var firstAsset, secondAsset Asset
		switch {
		// If the column first_asset has no address and the column first_asset_native contains true,
		// then we consider this token as a pool to TON
		case record[0] == "NULL" && record[2] == "true":
			firstAsset = Asset{Account: references.PTonV1}
			secondAccountID, err := ton.ParseAccountID(record[1])
			if err != nil {
				return Assets{}, err
			}
			secondAsset = Asset{Account: secondAccountID}
			// If the column second_asset has no address and the column second_asset_native contains true,
			// then we consider this token as a pool to TON
		case record[1] == "NULL" && record[3] != "true":
			firstAccountID, err := ton.ParseAccountID(record[0])
			if err != nil {
				return Assets{}, err
			}
			firstAsset = Asset{Account: firstAccountID}
			secondAsset = Asset{Account: references.PTonV1}
		default:
			// By default, we assume that the two assets are not paired with TON.
			// This could be a pair like a jetton to USDT or to other jettons
			firstAccountID, err := ton.ParseAccountID(record[0])
			if err != nil {
				return Assets{}, err
			}
			firstAsset = Asset{Account: firstAccountID}
			secondAccountID, err := ton.ParseAccountID(record[1])
			if err != nil {
				return Assets{}, err
			}
			secondAsset = Asset{Account: secondAccountID}
		}
		firstAsset.Reserve, err = strconv.ParseFloat(record[4], 64)
		if err != nil {
			return Assets{}, err
		}
		secondAsset.Reserve, err = strconv.ParseFloat(record[5], 64)
		if err != nil {
			return Assets{}, err
		}
		firstAsset.Decimals, err = parseDecimals(record[6])
		if err != nil {
			return Assets{}, err
		}
		secondAsset.Decimals, err = parseDecimals(record[7])
		if err != nil {
			return Assets{}, err
		}
		var isStable bool
		if record[8] == "true" {
			isStable = true
		}
		firstAsset.HoldersCount, err = strconv.Atoi(record[9])
		if err != nil {
			return Assets{}, err
		}
		secondAsset.HoldersCount, err = strconv.Atoi(record[10])
		if err != nil {
			return Assets{}, err
		}
		return Assets{Assets: []Asset{firstAsset, secondAsset}, IsStable: isStable}, nil
	}
	parseLpAsset := func(record []string, firstAsset, secondAsset Asset) (LpAsset, error) {
		lpAsset, err := ton.ParseAccountID(record[11])
		if err != nil {
			return LpAsset{}, err
		}
		if record[12] == "0" {
			return LpAsset{}, fmt.Errorf("unknown total supply")
		}
		totalSupply, ok := new(big.Int).SetString(record[12], 10)
		if !ok {
			return LpAsset{}, fmt.Errorf("failed to parse total supply")
		}
		decimals, err := strconv.Atoi(record[13])
		if err != nil {
			return LpAsset{}, err
		}
		return LpAsset{
			Account:     lpAsset,
			Decimals:    decimals,
			TotalSupply: totalSupply,
			Assets:      []Asset{firstAsset, secondAsset},
		}, nil
	}
	var actualAssets []Assets
	actualLpAssets := make(map[ton.AccountID]LpAsset)
	for idx, record := range records {
		if idx == 0 || len(record) < 14 { // Skip headers
			continue
		}
		assets, err := parseAssets(record)
		if err != nil {
			slog.Error("[convertedDedustPoolResponse] failed to parse assets", slog.Any("error", err))
			continue
		}
		firstAsset, secondAsset := assets.Assets[0], assets.Assets[1]
		if firstAsset.Reserve == 0 || secondAsset.Reserve == 0 {
			continue
		}
		actualAssets = append(actualAssets, assets)
		lpAsset, err := parseLpAsset(record, firstAsset, secondAsset)
		if err != nil {
			continue
		}
		actualLpAssets[lpAsset.Account] = lpAsset
	}

	return actualAssets, maps.Values(actualLpAssets), nil
}

func calculateLpAssetPrice(asset LpAsset, pools map[ton.AccountID]float64) float64 {
	firstAsset := asset.Assets[0]
	secondAsset := asset.Assets[1]
	firstAssetPrice, ok := pools[firstAsset.Account]
	if !ok {
		return 0
	}
	secondAssetPrice, ok := pools[secondAsset.Account]
	if !ok {
		return 0
	}
	// Adjust the reserves of assets considering their decimal places
	firstAssetAdjustedReserve := new(big.Float).Quo(
		big.NewFloat(firstAsset.Reserve),
		new(big.Float).SetFloat64(math.Pow(10, float64(firstAsset.Decimals))),
	)
	secondAssetAdjustedReserve := new(big.Float).Quo(
		big.NewFloat(secondAsset.Reserve),
		new(big.Float).SetFloat64(math.Pow(10, float64(secondAsset.Decimals))),
	)
	// Calculate the total value of the jetton by summing the values of both assets
	totalValue := new(big.Float).Add(
		new(big.Float).Mul(firstAssetAdjustedReserve, big.NewFloat(firstAssetPrice)),
		new(big.Float).Mul(secondAssetAdjustedReserve, big.NewFloat(secondAssetPrice)),
	)
	// Adjust the total supply for the asset decimals
	totalSupplyAdjusted := new(big.Float).Quo(
		new(big.Float).SetInt(asset.TotalSupply),
		new(big.Float).SetFloat64(math.Pow(10, float64(asset.Decimals))),
	)
	// Calculate the price of the jetton by dividing the total value by the adjusted total supply of tokens
	price := new(big.Float).Quo(totalValue, totalSupplyAdjusted)

	convertedPrice, _ := price.Float64()
	return convertedPrice
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
		minReserve := defaultMinReserve
		// Converting reserve prices to TON
		var updatedFirstAssetReserve float64
		if firstAsset.Decimals != defaultDecimals {
			updatedFirstAssetReserve = firstAsset.Reserve * math.Pow(10, float64(secondAsset.Decimals)-float64(firstAsset.Decimals)) * priceFirst
			// Recalculate minReserve to the decimals of the asset
			minReserve = defaultMinReserve * math.Pow(10, float64(secondAsset.Decimals)-defaultDecimals)
		} else {
			firstAsset.Reserve *= priceFirst
			updatedFirstAssetReserve = firstAsset.Reserve
		}
		if updatedFirstAssetReserve < minReserve {
			return ton.AccountID{}, 0
		}
		if secondAsset.HoldersCount < defaultMinHoldersCount {
			return ton.AccountID{}, 0
		}
		calculatedAccount = secondAsset.Account
		firstAssetDecimals, secondAssetDecimals = firstAsset.Decimals, secondAsset.Decimals
	}
	if okSecond { // Knowing the second asset's price, we determine the first asset's price
		minReserve := defaultMinReserve
		// Converting reserve prices to TON
		var updatedSecondAssetReserve float64
		if secondAsset.Decimals != defaultDecimals {
			updatedSecondAssetReserve = secondAsset.Reserve * math.Pow(10, float64(firstAsset.Decimals)-float64(secondAsset.Decimals)) * priceSecond
			// Recalculate minReserve to the decimals of the asset
			minReserve = defaultMinReserve * math.Pow(10, float64(firstAsset.Decimals)-defaultDecimals)
		} else {
			secondAsset.Reserve *= priceSecond
			updatedSecondAssetReserve = secondAsset.Reserve
		}
		if updatedSecondAssetReserve < minReserve {
			return ton.AccountID{}, 0
		}
		if firstAsset.HoldersCount < defaultMinHoldersCount {
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

func sendRequest(url, token string, headers http.Header) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header = headers
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", token))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}
