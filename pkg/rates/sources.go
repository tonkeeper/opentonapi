package rates

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"golang.org/x/exp/slog"
	"golang.org/x/net/context"
)

// GetCurrentRates fetches current jetton and fiat rates
func (m *Mock) GetCurrentRates() (map[string]float64, error) {
	const (
		tonstakers = "tonstakers"
		bemo       = "bemo"
		beetroot   = "beetroot"
		tsUSDe     = "tsUSDe"
		USDe       = "USDe"
		affUSDe    = "affUSDe"
		slpTokens  = "slp_tokens"
	)

	// Fetch market prices and calculate the median TON/USD rate
	marketTonPrices, err := m.GetCurrentMarketsTonPrice()
	if err != nil {
		return map[string]float64{}, err
	}
	medianTonPrice, err := getMedianPrice(marketTonPrices)
	if err != nil {
		return map[string]float64{}, err
	}

	// Fetch market prices and calculate the median TRX/USD rate
	marketTrxPrices, err := m.GetCurrentMarketsTrxPrice()
	if err != nil {
		return map[string]float64{}, err
	}
	medianTrxPrice, err := getMedianPrice(marketTrxPrices)
	if err != nil {
		return map[string]float64{}, err
	}

	// Retry helper to run a task up to 3 times with backoff
	retry := func(label string, tonPrice float64, pools map[ton.AccountID]float64,
		task func(context.Context, float64, map[ton.AccountID]float64) (map[ton.AccountID]float64, error),
	) (map[ton.AccountID]float64, error) {
		var err error
		var result map[ton.AccountID]float64
		for attempt := 1; attempt <= 3; attempt++ {
			result, err = task(context.Background(), tonPrice, pools)
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
		references.PTonV1:      1,
		references.PTonV2:      1,
		references.AffluentTon: 1,
	}

	// Fetch prices for special jettons
	priceFetchers := []struct {
		label string
		fetch func(context.Context, float64, map[ton.AccountID]float64) (map[ton.AccountID]float64, error)
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

	// Fetch and merge SLP tokens prices
	if slpPrices, err := retry(slpTokens, medianTonPrice, pools, m.getSlpTokensPrice); err == nil {
		for account, price := range slpPrices {
			pools[account] = price
		}
	}

	// Fetch and merge affluent USDe price
	if affUsdEPrice, err := retry(affUSDe, medianTonPrice, pools, m.getAffUsdEPrice); err == nil {
		for account, price := range affUsdEPrice {
			pools[account] = price
		}
	}

	// Compose final result map with fiat and jetton prices
	rates := map[string]float64{"TON": 1}
	// Add additional coins prices that absence in market rates
	additionalPrices := map[string]float64{"TRX": 1 / medianTrxPrice}
	for currency, price := range getFiatPrices(medianTonPrice, additionalPrices) {
		rates[currency] = price
	}
	for accountID, price := range pools {
		rates[accountID.ToRaw()] = price
	}

	return rates, nil
}

// getMedianPrice computes the median TON price from available market data
func getMedianPrice(markets []Market) (float64, error) {
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

// AddressKey is an address that can be used as a key in hashmap
type AddressKey struct {
	Prefix    tlb.Uint3
	Workchain tlb.Uint8
	Hash      tlb.Bits256
}

func (a AddressKey) Compare(other any) (int, bool) {
	otherAddr, ok := other.(AddressKey)
	if !ok {
		return 0, false
	}
	if !a.Prefix.Equal(otherAddr.Prefix) {
		return a.Prefix.Compare(otherAddr.Prefix)
	}
	if !a.Workchain.Equal(otherAddr.Workchain) {
		return a.Workchain.Compare(otherAddr.Workchain)
	}
	if !a.Hash.Equal(otherAddr.Hash) {
		return a.Hash.Compare(otherAddr.Hash)
	}
	return 0, true
}

func (a AddressKey) Equal(other any) bool {
	if otherAddr, ok := other.(AddressKey); ok {
		return a.Prefix.Equal(otherAddr.Prefix) && a.Workchain.Equal(otherAddr.Workchain) && a.Hash.Equal(otherAddr.Hash)
	}
	return false
}

func (a AddressKey) FixedSize() int {
	return 267
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
func (m *Mock) getBemoPrice(_ context.Context, _ float64, _ map[ton.AccountID]float64) (map[ton.AccountID]float64, error) {
	prices := make(map[ton.AccountID]float64)
	accounts := []ton.AccountID{references.BemoAccountOld, references.BemoAccountNew}
	headers := http.Header{"Content-Type": {"application/json"}}
	for _, account := range accounts {
		url := fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/get_full_data", account.ToRaw())
		respBody, err := sendRequest(url, m.TonApiToken, headers)
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
		prices[account] = price
	}

	return prices, nil
}

// getTonstakersPrice fetches Tonstakers price by smart contract data
func (m *Mock) getTonstakersPrice(_ context.Context, _ float64, _ map[ton.AccountID]float64) (map[ton.AccountID]float64, error) {
	url := fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/get_pool_full_data", references.TonstakersAccountPool.ToRaw())
	headers := http.Header{"Content-Type": {"application/json"}}
	respBody, err := sendRequest(url, m.TonApiToken, headers)
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
func (m *Mock) getBeetrootPrice(_ context.Context, tonPrice float64, _ map[ton.AccountID]float64) (map[ton.AccountID]float64, error) {
	if tonPrice == 0 {
		return nil, errors.New("unknown TON price")
	}
	contract := ton.MustParseAccountID("EQDC8MY5tY5rPM6KFFxz58fMUES6qSsFxi_Pbaig1QuO3F7y")
	account := ton.MustParseAccountID("EQAFGhmx199oH6kmL78PGBHyAx4d5CiJdfXwSjDK5F5IFyfC")

	url := fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/get_price_data", contract)
	headers := http.Header{"Content-Type": {"application/json"}}
	respBody, err := sendRequest(url, m.TonApiToken, headers)
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
func (m *Mock) getTsUSDePrice(_ context.Context, tonPrice float64, _ map[ton.AccountID]float64) (map[ton.AccountID]float64, error) {
	if tonPrice == 0 {
		return nil, errors.New("unknown TON price")
	}
	contract := ton.MustParseAccountID("EQChGuD1u0e7KUWHH5FaYh_ygcLXhsdG2nSHPXHW8qqnpZXW")
	account := ton.MustParseAccountID("EQDQ5UUyPHrLcQJlPAczd_fjxn8SLrlNQwolBznxCdSlfQwr")
	refShare := decimal.NewFromInt(1_000_000_000)

	url := fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/convertToAssets?args=%v", contract, refShare)
	headers := http.Header{"Content-Type": {"application/json"}}
	respBody, err := sendRequest(url, m.TonApiToken, headers)
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

func (m *Mock) getUsdEPrice(_ context.Context, tonPrice float64, _ map[ton.AccountID]float64) (map[ton.AccountID]float64, error) {
	if tonPrice == 0 {
		return nil, errors.New("unknown TON price")
	}
	account := ton.MustParseAccountID("EQAIb6KmdfdDR7CN1GBqVJuP25iCnLKCvBlJ07Evuu2dzP5f")

	url := "https://api.bybit.com/v5/market/tickers?category=spot&symbol=USDEUSDT"
	headers := http.Header{"Content-Type": {"application/json"}}
	respBody, err := sendRequest(url, "", headers)
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

func (m *Mock) getAffUsdEPrice(ctx context.Context, tonPrice float64, pools map[ton.AccountID]float64) (map[ton.AccountID]float64, error) {
	if tonPrice == 0 {
		return nil, errors.New("unknown TON price")
	}

	type GetVaultDataExecutionResult struct {
		Success  bool `json:"success"`
		ExitCode int  `json:"exit_code"`
		Decoded  struct {
			AffluentVaultData struct {
				Assets         boc.Cell   `json:"assets"`
				FactorialPools boc.Cell   `json:"factorial_pools"`
				TotalSupply    tlb.Int257 `json:"total_supply"`
			} `json:"affluent_vault_data"`
		} `json:"decoded"`
	}
	type Assets struct {
		TotalSupply  tlb.Int257     `json:"total_supply"`
		TotalBorrow  tlb.Int257     `json:"total_borrow"`
		SupplyShare  tlb.Int257     `json:"supply_share"`
		BorrowShare  tlb.Int257     `json:"borrow_share"`
		AssetAddress tlb.MsgAddress `json:"asset_address"`
	}
	type GetPoolDataExecutionResult struct {
		Success  bool `json:"success"`
		ExitCode int  `json:"exit_code"`
		Decoded  struct {
			Assets []Assets `json:"assets"`
		} `json:"decoded"`
	}
	type Metadata struct {
		Metadata struct {
			Symbol   string `json:"symbol"`
			Decimals string `json:"decimals"`
		} `json:"metadata"`
	}

	getAssetMetadata := func(rawAddress string) (Metadata, error) {
		url := fmt.Sprintf("https://tonapi.io/v2/jettons/%v", rawAddress)
		headers := http.Header{"Content-Type": {"application/json"}}
		respBody, err := sendRequest(url, m.TonApiToken, headers)
		if err != nil {
			return Metadata{}, err
		}
		var result Metadata
		if err = json.Unmarshal(respBody, &result); err != nil {
			return Metadata{}, err
		}
		return result, nil
	}

	isVaultAssets := func(metadata Metadata) (bool, error) {
		return strings.HasPrefix(metadata.Metadata.Symbol, references.AffluentAssetPrefix), nil
	}

	assetAddress := references.AffUSDeVault
	if ctx.Value("assetAddress") != nil {
		assetAddress = ctx.Value("assetAddress").(string)
	}
	url := fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/get_vault_data", assetAddress)
	headers := http.Header{"Content-Type": {"application/json"}}
	respBody, err := sendRequest(url, m.TonApiToken, headers)
	if err != nil {
		return nil, err
	}
	var result GetVaultDataExecutionResult
	if err = json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, errors.New("invalid data")
	}

	assetTotalSupply := big.Int(result.Decoded.AffluentVaultData.TotalSupply)
	assetTotalSupplyAmount, _ := assetTotalSupply.Float64()

	affMetadata, err := getAssetMetadata(references.AffUSDeVault)
	if err != nil {
		return nil, err
	}
	affDecimals, err := strconv.ParseInt(affMetadata.Metadata.Decimals, 10, 64)

	valueSum := 0.0
	var assets tlb.Hashmap[AddressKey, abi.AssetData]
	if err := tlb.Unmarshal(&result.Decoded.AffluentVaultData.Assets, &assets); err != nil {
		return nil, errors.New("invalid factorial pools")
	}

	for _, asset := range assets.Items() {
		rawAddress := fmt.Sprintf("%v:%v", asset.Key.Workchain, asset.Key.Hash.Hex())
		addr, err := ton.ParseAccountID(rawAddress)
		if err != nil {
			return nil, err
		}
		assetMetadata, err := getAssetMetadata(rawAddress)
		if err != nil {
			return nil, err
		}
		isVaultAsset, err := isVaultAssets(assetMetadata)
		if err != nil {
			return nil, err
		}
		var assetPrice float64
		if isVaultAsset {
			assetPrices, err := m.getAffUsdEPrice(
				context.WithValue(ctx, "assetAddress", rawAddress),
				tonPrice,
				pools,
			)
			if err != nil {
				return nil, err
			}
			assetPrice = assetPrices[addr]
		} else {
			assetPrice = pools[addr]
		}
		assetDecimals, err := strconv.ParseInt(assetMetadata.Metadata.Decimals, 10, 64)
		valueSum += float64(asset.Value.Cash) * assetPrice * math.Pow10(int(affDecimals-assetDecimals))
	}

	var factorialPools tlb.Hashmap[AddressKey, tlb.Maybe[tlb.Ref[tlb.Hashmap[AddressKey, abi.FactorialPoolAsset]]]]
	if err := tlb.Unmarshal(&result.Decoded.AffluentVaultData.FactorialPools, &factorialPools); err != nil {
		return nil, errors.New("invalid factorial pools")
	}

	for _, pool := range factorialPools.Items() {
		poolAddr := fmt.Sprintf("%v:%v", pool.Key.Workchain, pool.Key.Hash.Hex())
		url = fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/get_pool_data", poolAddr)
		headers = http.Header{"Content-Type": {"application/json"}}
		respBody, err = sendRequest(url, m.TonApiToken, headers)
		if err != nil {
			return nil, err
		}
		var poolResult GetPoolDataExecutionResult
		if err = json.Unmarshal(respBody, &poolResult); err != nil {
			return nil, err
		}
		if !poolResult.Success {
			return nil, errors.New("invalid data")
		}

		assetsFromPools := map[ton.AccountID]Assets{}
		for _, asset := range poolResult.Decoded.Assets {
			tlbAddr, err := tongo.AccountIDFromTlb(asset.AssetAddress)
			if err != nil {
				return nil, err
			}
			assetsFromPools[*tlbAddr] = asset
		}

		if !pool.Value.Exists {
			continue
		}
		for _, poolAsset := range pool.Value.Value.Value.Items() {
			rawAddress := fmt.Sprintf("%v:%v", poolAsset.Key.Workchain, poolAsset.Key.Hash.Hex())
			assetAddr, err := ton.ParseAccountID(rawAddress)
			if err != nil {
				return nil, err
			}

			assetFromPool := assetsFromPools[assetAddr]
			supplyShare := big.Int(assetFromPool.SupplyShare)
			totalSupplyShare, _ := supplyShare.Float64()
			totalSupply := big.Int(assetFromPool.TotalSupply)
			totalSupplyAmount, _ := totalSupply.Float64()
			supplyAmount := 0.0
			if poolAsset.Value.Supply > 0 && totalSupplyShare > 0 {
				supplyAmount = (float64(poolAsset.Value.Supply) * totalSupplyAmount) / totalSupplyShare
			}

			borrowShare := big.Int(assetFromPool.BorrowShare)
			totalBorrowShare, _ := borrowShare.Float64()
			totalBorrow := big.Int(assetFromPool.TotalBorrow)
			totalBorrowAmount, _ := totalBorrow.Float64()
			borrowAmount := 0.0
			if poolAsset.Value.Borrow > 0 && totalBorrowShare > 0 {
				borrowAmount = (float64(poolAsset.Value.Borrow) * totalBorrowAmount) / totalBorrowShare
			}
			netAmount := supplyAmount - borrowAmount

			assetMetadata, err := getAssetMetadata(rawAddress)
			if err != nil {
				return nil, err
			}
			isVaultAsset, err := isVaultAssets(assetMetadata)
			if err != nil {
				return nil, err
			}
			var assetPrice float64
			if isVaultAsset {
				assetPrices, err := m.getAffUsdEPrice(
					context.WithValue(ctx, "assetAddress", rawAddress),
					tonPrice,
					pools,
				)
				if err != nil {
					return nil, err
				}
				assetPrice = assetPrices[assetAddr]
			} else {
				assetPrice = pools[assetAddr]
			}
			assetDecimals, err := strconv.ParseInt(assetMetadata.Metadata.Decimals, 10, 64)
			valueSum += assetPrice * netAmount * math.Pow10(int(affDecimals-assetDecimals))
		}
	}

	return map[ton.AccountID]float64{
		ton.MustParseAccountID(references.AffUSDeVault): valueSum / assetTotalSupplyAmount,
	}, nil
}

// getSlpTokensPrice calculates SLP token prices
func (m *Mock) getSlpTokensPrice(_ context.Context, tonPrice float64, pools map[ton.AccountID]float64) (map[tongo.AccountID]float64, error) {
	if tonPrice == 0 {
		return nil, errors.New("unknown TON price")
	}

	result := make(map[tongo.AccountID]float64)
	for slpType, account := range references.SlpAccounts {
		url := fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/get_vault_data", account.ToRaw())
		headers := http.Header{"Content-Type": {"application/json"}}
		respBody, err := sendRequest(url, m.TonApiToken, headers)
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
		case references.JUsdtSlpType:
			result[references.JUsdtSlp] = val / tonPrice
		case references.UsdtSlpType:
			result[references.UsdtSlp] = val / tonPrice
		case references.TonSlpType:
			result[references.TonSlp] = val
		case references.NotSlpType:
			accountID := ton.MustParseAccountID("EQAvlWFDxGF2lXm67y4yzC17wYKD9A0guwPkMs1gOsM__NOT")
			if price, ok := pools[accountID]; ok {
				result[references.NotSlp] = price * val
			}
		}
	}

	return result, nil
}
