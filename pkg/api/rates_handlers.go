package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo/ton"
)

func (h *Handler) GetChartRates(ctx context.Context, params oas.GetChartRatesParams) (*oas.GetChartRatesOK, error) {
	var (
		token              string
		startDate, endDate *int64
	)
	if strings.ToUpper(params.Token) == "TON" {
		token = "TON"
	} else {
		accountID, err := ton.ParseAccountID(params.Token)
		if err != nil {
			return nil, toError(http.StatusBadRequest, err)
		}
		token = accountID.ToRaw()
	}
	if params.Currency.Set {
		params.Currency.Value = strings.ToUpper(params.Currency.Value)
	}
	if params.StartDate.Set {
		startDate = &params.StartDate.Value
	}
	if params.EndDate.Set {
		endDate = &params.EndDate.Value
	}
	charts, err := h.ratesSource.GetRatesChart(token, params.Currency.Value, startDate, endDate)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	bytesResp, err := json.Marshal(charts)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &oas.GetChartRatesOK{Points: bytesResp}, nil
}

func (h *Handler) GetRates(ctx context.Context, params oas.GetRatesParams) (*oas.GetRatesOK, error) {
	params.Tokens = strings.TrimSpace(params.Tokens)
	tokens := strings.Split(params.Tokens, ",")
	if len(tokens) == 0 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("tokens is required param"))
	}
	human := false // temporary kludge for keeper
	var convertedTokens []string
	for _, token := range tokens {
		if len(token) == 48 {
			human = true
		}
		if accountID, err := ton.ParseAccountID(token); err == nil {
			token = accountID.ToRaw()
		} else {
			token = strings.ToUpper(token)
		}
		convertedTokens = append(convertedTokens, token)
	}

	params.Currencies = strings.TrimSpace(params.Currencies)
	currencies := strings.Split(params.Currencies, ",")
	if len(currencies) == 0 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("currencies is required param"))
	}
	for i := range currencies {
		if len(currencies[i]) == 48 {
			human = true
		}
		if len(currencies[i]) < 30 { //not jetton
			currencies[i] = strings.ToUpper(currencies[i])
		}
	}
	if len(tokens) > 100 || len(currencies) > 100 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("max params limit is 100 items"))
	}

	todayRates, yesterdayRates, weekRates, monthRates, err := h.getRates()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}

	ratesRes := make(oas.TokenRates)
	for _, token := range convertedTokens {
		for _, currency := range currencies {
			if ratesRes, err = convertRates(ratesRes, token, currency, todayRates, yesterdayRates, weekRates, monthRates); err != nil {
				return nil, err
			}
		}
	}
	if human { // temporary kludge for keeper
		temp := make(oas.TokenRates, len(ratesRes))
		for k, v := range ratesRes {
			if len(k) > 48 {
				k = ton.MustParseAccountID(k).ToHuman(true, false)
			}
			temp[k] = v
		}
		ratesRes = temp
	}

	return &oas.GetRatesOK{Rates: ratesRes}, nil
}

func calculateConvertedPrice(rates map[string]float64, currency, token string) (float64, error) {
	currencyPrice, ok := rates[currency]
	if !ok {
		return 0, fmt.Errorf("invalid currency: %v", currency)
	}
	if currencyPrice == 0 {
		return 0, fmt.Errorf("price is zero")
	}
	tokenPrice, ok := rates[token]
	if !ok {
		return 0, fmt.Errorf("invalid token: %v", token)
	}
	return tokenPrice * (1 / currencyPrice), nil
}

func (h *Handler) getRates() (map[string]float64, map[string]float64, map[string]float64, map[string]float64, error) {
	today := time.Now().UTC()
	yesterday := today.AddDate(0, 0, -1).Unix()
	weekAgo := today.AddDate(0, 0, -7).Unix()
	monthAgo := today.AddDate(0, 0, -30).Unix()

	todayRates, err := h.ratesSource.GetRates(today.Unix())
	if err != nil {
		return map[string]float64{}, map[string]float64{}, map[string]float64{}, map[string]float64{}, err
	}
	yesterdayRates, err := h.ratesSource.GetRates(yesterday)
	if err != nil {
		return map[string]float64{}, map[string]float64{}, map[string]float64{}, map[string]float64{}, err
	}
	weekRates, err := h.ratesSource.GetRates(weekAgo)
	if err != nil {
		return map[string]float64{}, map[string]float64{}, map[string]float64{}, map[string]float64{}, err
	}
	monthRates, err := h.ratesSource.GetRates(monthAgo)
	if err != nil {
		return map[string]float64{}, map[string]float64{}, map[string]float64{}, map[string]float64{}, err
	}
	return todayRates, yesterdayRates, weekRates, monthRates, nil
}

func convertRates(rates oas.TokenRates, token, currency string, todayRates, yesterdayRates, weekRates, monthRates map[string]float64) (oas.TokenRates, error) {
	todayCurrencyPrice, ok := todayRates[currency]
	if !ok {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("invalid currency: %v", currency))
	}
	rate, ok := rates[token]
	if !ok {
		rate = oas.TokenRatesItem{Prices: oas.NewOptTokenRatesItemPrices(oas.TokenRatesItemPrices{}), Diff24h: oas.NewOptTokenRatesItemDiff24h(oas.TokenRatesItemDiff24h{}), Diff7d: oas.NewOptTokenRatesItemDiff7d(oas.TokenRatesItemDiff7d{}), Diff30d: oas.NewOptTokenRatesItemDiff30d(oas.TokenRatesItemDiff30d{})}
		rates[token] = rate
	}
	todayTokenPrice, ok := todayRates[token]
	if !ok {
		rate = oas.TokenRatesItem{Prices: oas.NewOptTokenRatesItemPrices(oas.TokenRatesItemPrices{}), Diff24h: oas.NewOptTokenRatesItemDiff24h(oas.TokenRatesItemDiff24h{}), Diff7d: oas.NewOptTokenRatesItemDiff7d(oas.TokenRatesItemDiff7d{}), Diff30d: oas.NewOptTokenRatesItemDiff30d(oas.TokenRatesItemDiff30d{})}
		return rates, nil
	}
	convertedTodayPrice := todayTokenPrice * (1 / todayCurrencyPrice)
	rate.Prices.Value[currency] = convertedTodayPrice

	var diff24h, diff7w, diff1m float64
	if convertedYesterdayPrice, _ := calculateConvertedPrice(yesterdayRates, currency, token); convertedYesterdayPrice != 0 {
		diff24h = ((convertedTodayPrice - convertedYesterdayPrice) / convertedYesterdayPrice) * 100
	}
	if convertedWeekPrice, _ := calculateConvertedPrice(weekRates, currency, token); convertedWeekPrice != 0 {
		diff7w = ((convertedTodayPrice - convertedWeekPrice) / convertedWeekPrice) * 100
	}
	if convertedMonthPrice, _ := calculateConvertedPrice(monthRates, currency, token); convertedMonthPrice != 0 {
		diff1m = ((convertedTodayPrice - convertedMonthPrice) / convertedMonthPrice) * 100
	}

	diff24h = math.Round(diff24h*100) / 100
	diff7w = math.Round(diff7w*100) / 100
	diff1m = math.Round(diff1m*100) / 100

	switch true {
	case diff24h < 0:
		rate.Diff24h.Value[currency] = fmt.Sprintf("%.2f%%", diff24h)
	case diff24h > 0:
		rate.Diff24h.Value[currency] = fmt.Sprintf("+%.2f%%", diff24h)
	default:
		rate.Diff24h.Value[currency] = "0"
	}

	switch true {
	case diff7w < 0:
		rate.Diff7d.Value[currency] = fmt.Sprintf("%.2f%%", diff7w)
	case diff7w > 0:
		rate.Diff7d.Value[currency] = fmt.Sprintf("+%.2f%%", diff7w)
	default:
		rate.Diff7d.Value[currency] = "0"
	}

	switch true {
	case diff1m < 0:
		rate.Diff30d.Value[currency] = fmt.Sprintf("%.2f%%", diff1m)
	case diff1m > 0:
		rate.Diff30d.Value[currency] = fmt.Sprintf("+%.2f%%", diff1m)
	default:
		rate.Diff30d.Value[currency] = "0"
	}

	return rates, nil
}
