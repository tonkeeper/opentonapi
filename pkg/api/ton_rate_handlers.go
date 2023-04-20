package api

import (
	"context"
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"strconv"
	"strings"
)

func (h *Handler) GetTonRate(ctx context.Context, params oas.GetTonRateParams) (res oas.GetTonRateRes, err error) {
	currency := strings.ToUpper(params.Currency)
	rates := h.tonRates.GetRates()
	btcInFiat, ok := rates[currency]
	btcPrice := rates["TON"]
	if !ok {
		return &oas.BadRequest{Error: "invalid currency: " + currency}, nil
	}
	btcInFiatConverted, err := strconv.ParseFloat(btcInFiat, 64)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	btcPriceConverted, err := strconv.ParseFloat(btcPrice, 64)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	decimalBTCInFiat := decimal.NewFromFloat(btcInFiatConverted)
	decimalBTCPrice := decimal.NewFromFloat(btcPriceConverted)

	tonPrice, _ := decimalBTCInFiat.Div(decimalBTCPrice).Round(5).Float64()

	return &oas.TonRate{Price: tonPrice, Currency: currency}, nil
}
