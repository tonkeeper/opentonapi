package rates

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
)

func (m *Mock) GetCurrentRates() (map[string]float64, error) {
	// List of special jettons for which prices are calculated based on contract's GET methods
	const (
		tonstakers string = "tonstakers"
		bemo       string = "bemo"
		slpTokens  string = "slp_tokens"
	)

	marketsPrice, err := m.GetCurrentMarketsTonPrice()
	if err != nil {
		return map[string]float64{}, err
	}
	medianTonPriceToUsd, err := getMedianTonPrice(marketsPrice)
	if err != nil {
		return map[string]float64{}, err
	}
	pools := m.getPools()
	fiatPrices := getFiatPrices(medianTonPriceToUsd)

	retry := func(label string, tonPrice float64, task func(tonPrice float64) (map[ton.AccountID]float64, error)) (map[ton.AccountID]float64, error) {
		for attempt := 0; attempt < 3; attempt++ {
			accountsPrice, err := task(tonPrice)
			if err != nil {
				zap.Error(fmt.Errorf("label %v, attempt %v, failed to get account price: %v", label, attempt+1, err))
				errorsCounter.WithLabelValues(label).Inc()
				time.Sleep(time.Second * 1)
				continue
			}
			return accountsPrice, nil
		}
		return nil, fmt.Errorf("attempts failed")
	}

	if tonstakersPrice, err := retry(tonstakers, medianTonPriceToUsd, m.getTonstakersPrice); err == nil {
		for account, price := range tonstakersPrice {
			pools[account] = price
		}
	}
	if bemoPrice, err := retry(bemo, medianTonPriceToUsd, m.getBemoPrice); err == nil {
		for account, price := range bemoPrice {
			pools[account] = price
		}
	}
	if slpTokensPrice, err := retry(slpTokens, medianTonPriceToUsd, m.getSlpTokensPrice); err == nil {
		for account, price := range slpTokensPrice {
			pools[account] = price
		}
	}
	rates := make(map[string]float64) // Includes prices for jettons as well as for fiat currencies
	rates["TON"] = 1
	for currency, price := range fiatPrices {
		rates[currency] = price
	}
	for account, price := range pools {
		rates[account.ToRaw()] = price
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
	if length%2 == 0 { // If the array length is even, return the average of the two middle elements
		middle1 := prices[length/2-1]
		middle2 := prices[length/2]
		return (middle1 + middle2) / 2, nil
	}
	// If the array length is odd, return the middle element
	return prices[length/2], nil
}

// getBemoPrice retrieves the price of the Bemo jetton from the contract
// TonApi is used because the standard liteserver executor cannot invoke methods on the account
func (m *Mock) getBemoPrice(tonPrice float64) (map[ton.AccountID]float64, error) {
	url := fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/get_full_data", references.BemoAccount.ToRaw())
	respBody, err := sendRequest(url, m.TonApiToken)
	if err != nil {
		return map[ton.AccountID]float64{}, err
	}
	defer respBody.Close()
	type fullData struct {
		Success bool `json:"success"`
		Stack   []struct {
			Num string `json:"num"`
		}
	}
	var result fullData
	if err = json.NewDecoder(respBody).Decode(&result); err != nil {
		return map[ton.AccountID]float64{}, fmt.Errorf("[getBemoPrice] failed to decode response: %v", err)
	}
	if !result.Success {
		return map[ton.AccountID]float64{}, fmt.Errorf("not success")
	}
	if len(result.Stack) < 2 {
		return map[ton.AccountID]float64{}, fmt.Errorf("empty stack")
	}
	firstParam, err := strconv.ParseInt(result.Stack[0].Num, 0, 64)
	if err != nil {
		return map[ton.AccountID]float64{}, err
	}
	secondParam, err := strconv.ParseInt(result.Stack[1].Num, 0, 64)
	if err != nil {
		return map[ton.AccountID]float64{}, err
	}
	price := float64(secondParam) / float64(firstParam)

	return map[ton.AccountID]float64{references.BemoAccount: price}, nil
}

// getTonstakersPrice retrieves the price and token address of an account in the Tonstakers pool
// TonApi is used because the standard liteserver executor cannot invoke methods on the account
func (m *Mock) getTonstakersPrice(tonPrice float64) (map[ton.AccountID]float64, error) {
	url := fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/get_pool_full_data", references.TonstakersAccountPool.ToRaw())
	respBody, err := sendRequest(url, m.TonApiToken)
	if err != nil {
		return map[ton.AccountID]float64{}, err
	}
	defer respBody.Close()
	type poolFullData struct {
		Success bool `json:"success"`
		Decoded struct {
			JettonMinter    string `json:"jetton_minter"`
			ProjectBalance  int64  `json:"projected_balance"`
			ProjectedSupply int64  `json:"projected_supply"`
		}
	}
	var result poolFullData
	if err = json.NewDecoder(respBody).Decode(&result); err != nil {
		return map[ton.AccountID]float64{}, fmt.Errorf("[getTonstakersPrice] failed to decode response: %v", err)
	}
	if !result.Success {
		return map[ton.AccountID]float64{}, fmt.Errorf("not success")
	}
	if result.Decoded.ProjectBalance == 0 || result.Decoded.ProjectedSupply == 0 {
		return map[ton.AccountID]float64{}, fmt.Errorf("empty balance")
	}
	account, err := ton.ParseAccountID(result.Decoded.JettonMinter)
	if err != nil {
		return map[ton.AccountID]float64{}, err
	}
	price := float64(result.Decoded.ProjectBalance) / float64(result.Decoded.ProjectedSupply)

	return map[ton.AccountID]float64{account: price}, nil
}

// getSlpTokensPrice retrieves the price of SLP jettons from the contract
// TonApi is used because the standard liteserver executor cannot invoke methods on the account
func (m *Mock) getSlpTokensPrice(tonPrice float64) (map[tongo.AccountID]float64, error) {
	type vaultData struct {
		Success bool `json:"success"`
		Stack   []struct {
			Type string `json:"type"`
			Num  string `json:"num"`
		} `json:"stack"`
	}
	accountsPrice := make(map[tongo.AccountID]float64)
	for slpType, account := range references.SlpAccounts {
		url := fmt.Sprintf("https://tonapi.io/v2/blockchain/accounts/%v/methods/get_vault_data", account.ToRaw())
		respBody, err := sendRequest(url, m.TonApiToken)
		if err != nil {
			continue
		}
		var result vaultData
		if err = json.NewDecoder(respBody).Decode(&result); err != nil {
			respBody.Close()
			return nil, err
		}
		respBody.Close()
		if !result.Success {
			return nil, fmt.Errorf("not success")
		}
		multiplier, err := strconv.ParseInt(result.Stack[1].Num, 0, 64)
		if err != nil {
			return nil, err
		}
		if multiplier == 0 {
			return nil, fmt.Errorf("unknown price")
		}
		switch slpType {
		case references.JUsdtSlpType:
			usdPrice := float64(multiplier) / float64(ton.OneTON)
			accountsPrice[references.JUsdtSlp] = usdPrice / tonPrice
		case references.UsdtSlpType:
			usdPrice := float64(multiplier) / float64(ton.OneTON)
			accountsPrice[references.UsdtSlp] = usdPrice / tonPrice
		case references.TonSlpType:
			usdPrice := tonPrice * (float64(multiplier) / float64(ton.OneTON))
			accountsPrice[references.TonSlp] = usdPrice / tonPrice
		}
	}

	return accountsPrice, nil
}
