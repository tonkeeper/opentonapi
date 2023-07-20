package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func (h *Handler) GetRates(ctx context.Context, params oas.GetRatesParams) (res oas.GetRatesRes, err error) {
	params.Tokens = strings.TrimSpace(params.Tokens)
	tokens := strings.Split(params.Tokens, ",")
	if len(tokens) == 0 {
		return &oas.BadRequest{"tokens is required param"}, nil
	}

	params.Currencies = strings.TrimSpace(strings.ToUpper(params.Currencies))
	currencies := strings.Split(params.Currencies, ",")
	if len(currencies) == 0 {
		return &oas.BadRequest{"currencies is required param"}, nil
	}

	if len(tokens) > 50 || len(currencies) > 50 {
		return &oas.BadRequest{"max params limit is 50 items"}, nil
	}

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)
	todayRates, err := h.ratesSource.GetRates(now)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	yesterdayRates, err := h.ratesSource.GetRates(yesterday)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}

	type tokenRates struct {
		Prices  map[string]float64 `json:"prices"`
		Diff24h map[string]string  `json:"diff_24h"`
	}

	ratesRes := make(map[string]tokenRates)
	for _, token := range tokens {
		if token == "ton" {
			token = "TON"
		}
		for _, currency := range currencies {
			todayCurrencyPrice, ok := todayRates[currency]
			if !ok {
				return &oas.BadRequest{fmt.Sprintf("invalid currency: %v", currency)}, nil
			}
			rate, ok := ratesRes[token]
			if !ok {
				rate = tokenRates{Prices: map[string]float64{}, Diff24h: map[string]string{}}
				ratesRes[token] = rate
			}
			todayTokenPrice, ok := todayRates[token]
			if !ok {
				ratesRes[token] = tokenRates{Prices: map[string]float64{}, Diff24h: map[string]string{}}
				continue
			}

			var convertedTodayPrice, convertedYesterdayPrice, diff float64
			if todayTokenPrice != 0 {
				convertedTodayPrice = (1 / todayTokenPrice) * todayCurrencyPrice
			}
			rate.Prices[currency] = convertedTodayPrice

			if yesterdayRates[token] != 0 {
				convertedYesterdayPrice = (1 / yesterdayRates[token]) * yesterdayRates[currency]
			}
			if convertedYesterdayPrice != 0 {
				diff = ((convertedTodayPrice - convertedYesterdayPrice) / convertedYesterdayPrice) * 100
			}

			diff = math.Round(diff*100) / 100
			switch true {
			case diff < 0:
				rate.Diff24h[currency] = fmt.Sprintf("%.2f%%", diff)
			case diff > 0:
				rate.Diff24h[currency] = fmt.Sprintf("+%.2f%%", diff)
			default:
				rate.Diff24h[currency] = "0"
			}
		}
	}

	bytesRatesRes, err := json.Marshal(ratesRes)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}

	return &oas.GetRatesOK{Rates: bytesRatesRes}, nil
}
