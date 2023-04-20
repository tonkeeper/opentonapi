package api

import (
	"context"
	"encoding/json"
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"strconv"
	"strings"
)

func (h *Handler) GetRates(ctx context.Context, params oas.GetRatesParams) (res oas.GetRatesRes, err error) {
	params.Currencies = strings.TrimSpace(params.Currencies)
	currencies := strings.Split(params.Currencies, ",")
	if len(currencies) == 0 {
		return &oas.BadRequest{"currencies is required param"}, nil
	}

	params.In = strings.TrimSpace(params.In)
	in := strings.Split(params.In, ",")
	if len(in) == 0 {
		return &oas.BadRequest{"in is required param"}, nil
	}

	rates := h.tonRates.GetRates()
	btcPrice := rates["TON"]
	btcPriceConverted, err := strconv.ParseFloat(btcPrice, 64)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	decimalBTCPrice := decimal.NewFromFloat(btcPriceConverted)

	ratesRes := make(map[string]map[string]map[string]interface{})
	for _, currency := range currencies {
		switch currency {
		case "ton":
			for _, needConverted := range in {
				if needConverted == "ton" {
					continue
				}
				btcInFiat, ok := rates[strings.ToUpper(needConverted)]
				if !ok {
					return &oas.BadRequest{Error: "invalid currency: " + needConverted}, nil
				}
				btcInFiatConverted, err := strconv.ParseFloat(btcInFiat, 64)
				if err != nil {
					return &oas.InternalError{Error: err.Error()}, nil
				}
				decimalBTCInFiat := decimal.NewFromFloat(btcInFiatConverted)
				tonPrice, _ := decimalBTCInFiat.Div(decimalBTCPrice).Round(5).Float64()
				ton, ok := ratesRes["ton"]
				if !ok {
					ratesRes["ton"] = map[string]map[string]interface{}{"prices": {needConverted: tonPrice}}
					continue
				}
				prices, _ := ton["prices"]
				prices[needConverted] = tonPrice
				ton["prices"] = prices
				ratesRes["ton"] = ton
			}
		}
	}

	bytesRatesRes, err := json.Marshal(ratesRes)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}

	return &oas.GetRatesOK{Rates: bytesRatesRes}, nil
}
