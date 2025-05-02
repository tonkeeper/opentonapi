package rates

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
	"golang.org/x/exp/slog"
)

// GetCurrentRates fetches current jetton and fiat rates
func (m *Mock) GetCurrentRates() (map[string]float64, error) {
	const (
		tonstakers = "tonstakers"
		bemo       = "bemo"
		beetroot   = "beetroot"
		tsUSDe     = "tsUSDe"
		USDe       = "USDe"
		slpTokens  = "slp_tokens"
	)

	// Fetch market prices and calculate the median TON/USD rate
	marketPrices, err := m.GetCurrentMarketsTonPrice()
	if err != nil {
		return map[string]float64{}, err
	}
	medianTonPrice, err := getMedianTonPrice(marketPrices)
	if err != nil {
		return map[string]float64{}, err
	}

	// Retry helper to run a task up to 3 times with backoff
	retry := func(label string, tonPrice float64, pools map[ton.AccountID]float64,
		task func(float64, map[ton.AccountID]float64) (map[ton.AccountID]float64, error),
	) (map[ton.AccountID]float64, error) {
		var err error
		var result map[ton.AccountID]float64
		for attempt := 1; attempt <= 3; attempt++ {
			result, err = task(tonPrice, pools)
			if err == nil {
				return result, nil
			}
			errorsCounter.WithLabelValues(label).Inc()
			time.Sleep(time.Second)
		}
		slog.Error("failed to get account price", slog.String("label", label), slog.Any("error", err))
		return nil, errors.New("failed to get account price")
	}

	// Base pools initialized
	pools := map[ton.AccountID]float64{
		references.PTonV1: 1,
		references.PTonV2: 1,
	}

	// Fetch prices for special jettons
	priceFetchers := []struct {
		label string
		fetch func(float64, map[ton.AccountID]float64) (map[ton.AccountID]float64, error)
	}{
		{tonstakers, m.getTonstakersPrice},
		{bemo, m.getBemoPrice},
		{beetroot, m.getBeetrootPrice},
		{tsUSDe, m.getTsUSDePrice},
		{USDe, m.getUsdEPrice},
	}
	for _, pf := range priceFetchers {
		result, err := retry(pf.label, medianTonPrice, pools, pf.fetch)
		if err != nil || result == nil {
			continue
		}
		for account, price := range result {
			pools[account] = price
		}
	}

	// Fetch and merge prices for jettons from DEX
	pools = m.getJettonPricesFromDex(pools)

	// Fetch and merge SLP token prices
	if slpPrices, err := retry(slpTokens, medianTonPrice, pools, m.getSlpTokensPrice); err == nil {
		for account, price := range slpPrices {
			pools[account] = price
		}
	}

	// Compose final result map with fiat and jetton prices
	rates := map[string]float64{"TON": 1}
	for currency, price := range getFiatPrices(medianTonPrice) {
		rates[currency] = price
	}
	for accountID, price := range pools {
		rates[accountID.ToRaw()] = price
	}

	return rates, nil
}

// getMedianTonPrice computes the median TON price from available market data
func getMedianTonPrice(markets []Market) (float64, error) {
	// Extract the USD prices from the market data
	var prices []float64
	for _, market := range markets {
		prices = append(prices, market.UsdPrice)
	}
	// Sort the prices in ascending order
	sort.Float64s(prices)
	n := len(prices)
	if n == 0 {
		return 0, fmt.Errorf("no prices available")
	}
	if n%2 == 0 {
		// If the number of prices is even, return the average of the two middle prices
		return (prices[n/2-1] + prices[n/2]) / 2, nil
	}
	// If the number of prices is odd, return the middle price
	return prices[n/2], nil
}

// ExecutionResult represents a result from a smart contract call
type ExecutionResult struct {
	Success  bool `json:"success"`
	ExitCode int  `json:"exit_code"`
	Stack    []struct {
		Type  string `json:"type"`
		Cell  string `json:"cell"`
		Slice string `json:"slice"`
		Num   string `json:"num"`
	} `json:"stack"`
}

// getBemoPrice fetches BEMO price by smart contract data
func (m *Mock) getBemoPrice(_ float64, _ map[ton.AccountID]float64) (map[ton.AccountID]float64, error) {
	url := fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/get_full_data", references.BemoAccount.ToRaw())
	respBody, err := sendRequest(url, m.TonApiToken)
	if err != nil {
		return nil, err
	}
	var result ExecutionResult
	if err = json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if !result.Success || len(result.Stack) < 2 {
		return nil, errors.New("invalid data")
	}
	first, err := strconv.ParseInt(result.Stack[0].Num, 0, 64)
	if err != nil {
		return nil, err
	}
	second, err := strconv.ParseInt(result.Stack[1].Num, 0, 64)
	if err != nil {
		return nil, err
	}
	price := float64(second) / float64(first)

	return map[ton.AccountID]float64{references.BemoAccount: price}, nil
}

// getTonstakersPrice fetches Tonstakers price by smart contract data
func (m *Mock) getTonstakersPrice(_ float64, _ map[ton.AccountID]float64) (map[ton.AccountID]float64, error) {
	url := fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/get_pool_full_data", references.TonstakersAccountPool.ToRaw())
	respBody, err := sendRequest(url, m.TonApiToken)
	if err != nil {
		return nil, err
	}
	var result struct {
		Success bool `json:"success"`
		Decoded struct {
			JettonMinter    string `json:"jetton_minter"`
			ProjectBalance  int64  `json:"projected_balance"`
			ProjectedSupply int64  `json:"projected_supply"`
		}
	}
	if err = json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if !result.Success || result.Decoded.ProjectBalance == 0 || result.Decoded.ProjectedSupply == 0 {
		return nil, errors.New("invalid data")
	}
	account, err := ton.ParseAccountID(result.Decoded.JettonMinter)
	if err != nil {
		return nil, err
	}
	price := float64(result.Decoded.ProjectBalance) / float64(result.Decoded.ProjectedSupply)

	return map[ton.AccountID]float64{account: price}, nil
}

// getTonstakersPrice fetches Beetroot price by smart contract data
func (m *Mock) getBeetrootPrice(tonPrice float64, _ map[ton.AccountID]float64) (map[ton.AccountID]float64, error) {
	if tonPrice == 0 {
		return nil, errors.New("unknown TON price")
	}
	contract := ton.MustParseAccountID("EQDC8MY5tY5rPM6KFFxz58fMUES6qSsFxi_Pbaig1QuO3F7y")
	account := ton.MustParseAccountID("EQAFGhmx199oH6kmL78PGBHyAx4d5CiJdfXwSjDK5F5IFyfC")

	url := fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/get_price_data", contract)
	respBody, err := sendRequest(url, m.TonApiToken)
	if err != nil {
		return nil, err
	}
	var result ExecutionResult
	if err = json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if !result.Success || len(result.Stack) == 0 {
		return nil, errors.New("invalid data")
	}
	val, err := strconv.ParseInt(result.Stack[0].Num, 0, 64)
	if err != nil {
		return nil, err
	}
	price := (float64(val) / 100) / tonPrice

	return map[ton.AccountID]float64{account: price}, nil
}

// getTonstakersPrice fetches tsUSDe price by smart contract data
func (m *Mock) getTsUSDePrice(tonPrice float64, _ map[ton.AccountID]float64) (map[ton.AccountID]float64, error) {
	if tonPrice == 0 {
		return nil, errors.New("unknown TON price")
	}
	contract := ton.MustParseAccountID("EQChGuD1u0e7KUWHH5FaYh_ygcLXhsdG2nSHPXHW8qqnpZXW")
	account := ton.MustParseAccountID("EQDQ5UUyPHrLcQJlPAczd_fjxn8SLrlNQwolBznxCdSlfQwr")
	refShare := decimal.NewFromInt(1_000_000_000)

	url := fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/convertToAssets?args=%v", contract, refShare)
	respBody, err := sendRequest(url, m.TonApiToken)
	if err != nil {
		return nil, err
	}
	var result ExecutionResult
	if err = json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if !result.Success || len(result.Stack) != 1 || result.Stack[0].Type != "num" {
		return nil, errors.New("invalid data")
	}
	bigVal, ok := new(big.Int).SetString(strings.TrimPrefix(result.Stack[0].Num, "0x"), 16)
	if !ok {
		return nil, errors.New("invalid numeric value")
	}
	assets := decimal.NewFromBigInt(bigVal, 0)
	price := assets.Div(refShare).Div(decimal.NewFromFloat(tonPrice)).InexactFloat64()

	return map[ton.AccountID]float64{account: price}, nil
}

func (m *Mock) getUsdEPrice(tonPrice float64, _ map[ton.AccountID]float64) (map[ton.AccountID]float64, error) {
	if tonPrice == 0 {
		return nil, errors.New("unknown TON price")
	}
	account := ton.MustParseAccountID("EQAIb6KmdfdDR7CN1GBqVJuP25iCnLKCvBlJ07Evuu2dzP5f")

	url := "https://api.bybit.com/v5/market/tickers?category=spot&symbol=USDEUSDT"
	respBody, err := sendRequest(url, "")
	if err != nil {
		return nil, err
	}
	result := struct {
		RetMsg string `json:"retMsg"`
		Result struct {
			List []struct {
				LastPrice string `json:"lastPrice"`
			} `json:"list"`
		}
	}{}
	if err = json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if result.RetMsg != "OK" || len(result.Result.List) == 0 {
		return nil, errors.New("invalid data")
	}
	val, err := strconv.ParseFloat(result.Result.List[0].LastPrice, 64)
	if err != nil {
		return nil, err
	}
	price := val / tonPrice

	return map[ton.AccountID]float64{account: price}, nil
}

// getSlpTokensPrice calculates SLP token prices
func (m *Mock) getSlpTokensPrice(tonPrice float64, pools map[ton.AccountID]float64) (map[tongo.AccountID]float64, error) {
	if tonPrice == 0 {
		return nil, errors.New("unknown TON price")
	}

	result := make(map[tongo.AccountID]float64)
	for slpType, account := range references.SlpAccounts {
		url := fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/get_vault_data", account.ToRaw())
		respBody, err := sendRequest(url, m.TonApiToken)
		if err != nil {
			continue
		}
		var data ExecutionResult
		if err = json.Unmarshal(respBody, &data); err != nil {
			continue
		}
		if len(data.Stack) < 2 {
			continue
		}
		multiplier, err := strconv.ParseInt(data.Stack[1].Num, 0, 64)
		if err != nil || multiplier == 0 {
			continue
		}
		val := float64(multiplier) / float64(ton.OneTON)

		switch slpType {
		case references.JUsdtSlpType, references.UsdtSlpType:
			result[account] = val / tonPrice
		case references.TonSlpType:
			result[account] = val
		case references.NotSlpType:
			accountID := ton.MustParseAccountID("EQAvlWFDxGF2lXm67y4yzC17wYKD9A0guwPkMs1gOsM__NOT")
			if price, ok := pools[accountID]; ok {
				result[account] = price * val
			}
		}
	}

	return result, nil
}
