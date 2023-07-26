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
)

func (h *Handler) GetRates(ctx context.Context, params oas.GetRatesParams) (*oas.GetRatesOK, error) {
	params.Tokens = strings.TrimSpace(params.Tokens)
	tokens := strings.Split(params.Tokens, ",")
	if len(tokens) == 0 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("tokens is required param"))
	}

	params.Currencies = strings.TrimSpace(strings.ToUpper(params.Currencies))
	currencies := strings.Split(params.Currencies, ",")
	if len(currencies) == 0 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("currencies is required param"))
	}

	if len(tokens) > 50 || len(currencies) > 50 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("max params limit is 50 items"))
	}

	today := time.Now().UTC()
	yesterday := today.AddDate(0, 0, -1)
	weekAgo := today.AddDate(0, 0, -7)
	monthAgo := today.AddDate(0, 0, -30)

	todayRates, err := h.ratesSource.GetRates(today)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	yesterdayRates, err := h.ratesSource.GetRates(yesterday)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	weekRates, err := h.ratesSource.GetRates(weekAgo)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	monthRates, err := h.ratesSource.GetRates(monthAgo)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}

	type tokenRates struct {
		Prices  map[string]float64 `json:"prices"`
		Diff24h map[string]string  `json:"diff_24h"`
		Diff7d  map[string]string  `json:"diff_7d"`
		Diff30d map[string]string  `json:"diff_30d"`
	}

	ratesRes := make(map[string]tokenRates)
	for _, token := range tokens {
		if token == "ton" {
			token = "TON"
		}
		for _, currency := range currencies {
			todayCurrencyPrice, ok := todayRates[currency]
			if !ok {
				return nil, toError(http.StatusBadRequest, fmt.Errorf("invalid currency: %v", currency))
			}
			rate, ok := ratesRes[token]
			if !ok {
				rate = tokenRates{Prices: map[string]float64{}, Diff24h: map[string]string{}, Diff7d: map[string]string{}, Diff30d: map[string]string{}}
				ratesRes[token] = rate
			}
			todayTokenPrice, ok := todayRates[token]
			if !ok {
				ratesRes[token] = tokenRates{Prices: map[string]float64{}, Diff24h: map[string]string{}, Diff7d: map[string]string{}, Diff30d: map[string]string{}}
				continue
			}

			var convertedTodayPrice, convertedYesterdayPrice, diff24h, diff7w, diff1m float64
			if todayTokenPrice != 0 {
				convertedTodayPrice = (1 / todayTokenPrice) * todayCurrencyPrice
			}
			rate.Prices[currency] = convertedTodayPrice

			convertedYesterdayPrice, _ = calculateConvertedPrice(yesterdayRates, currency, token)
			convertedWeekPrice, _ := calculateConvertedPrice(weekRates, currency, token)
			convertedMonthPrice, _ := calculateConvertedPrice(monthRates, currency, token)

			if convertedYesterdayPrice != 0 {
				diff24h = ((convertedTodayPrice - convertedYesterdayPrice) / convertedYesterdayPrice) * 100
			}
			if convertedWeekPrice != 0 {
				diff7w = ((convertedTodayPrice - convertedWeekPrice) / convertedWeekPrice) * 100
			}
			if convertedMonthPrice != 0 {
				diff1m = ((convertedTodayPrice - convertedMonthPrice) / convertedMonthPrice) * 100
			}

			diff24h = math.Round(diff24h*100) / 100
			diff7w = math.Round(diff7w*100) / 100
			diff1m = math.Round(diff1m*100) / 100

			switch true {
			case diff24h < 0:
				rate.Diff24h[currency] = fmt.Sprintf("%.2f%%", diff24h)
			case diff24h > 0:
				rate.Diff24h[currency] = fmt.Sprintf("+%.2f%%", diff24h)
			default:
				rate.Diff24h[currency] = "0"
			}

			switch true {
			case diff7w < 0:
				rate.Diff7d[currency] = fmt.Sprintf("%.2f%%", diff7w)
			case diff7w > 0:
				rate.Diff7d[currency] = fmt.Sprintf("+%.2f%%", diff7w)
			default:
				rate.Diff7d[currency] = "0"
			}

			switch true {
			case diff1m < 0:
				rate.Diff30d[currency] = fmt.Sprintf("%.2f%%", diff1m)
			case diff1m > 0:
				rate.Diff30d[currency] = fmt.Sprintf("+%.2f%%", diff1m)
			default:
				rate.Diff30d[currency] = "0"
			}
		}
	}

	bytesRatesRes, err := json.Marshal(ratesRes)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}

	return &oas.GetRatesOK{Rates: bytesRatesRes}, nil
}

func calculateConvertedPrice(rates map[string]float64, currency, token string) (float64, error) {
	currencyPrice, ok := rates[currency]
	if !ok {
		return 0, fmt.Errorf("invalid currency: %v", currency)
	}
	tokenPrice, ok := rates[token]
	if !ok {
		return 0, fmt.Errorf("invalid token: %v", token)
	}
	if tokenPrice != 0 {
		return (1 / tokenPrice) * currencyPrice, nil
	}
	return 0, nil
}
