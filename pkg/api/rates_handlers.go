package api

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
)

const minTonAddressLength = 48

func (h *Handler) GetMarketsRates(ctx context.Context) (*oas.GetMarketsRatesOK, error) {
	markets, err := h.ratesSource.GetMarketsTonPrice()
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	converted := make([]oas.MarketTonRates, 0, len(markets))
	for _, market := range markets {
		converted = append(converted, oas.MarketTonRates{
			Market:         market.Name,
			UsdPrice:       market.UsdPrice,
			LastDateUpdate: market.DateUpdate.Unix(),
		})
	}
	return &oas.GetMarketsRatesOK{Markets: converted}, nil
}

func (h *Handler) GetChartRates(ctx context.Context, params oas.GetChartRatesParams) (*oas.GetChartRatesOK, error) {
	token := params.Token
	if len(token) >= minTonAddressLength {
		account, err := tongo.ParseAddress(token)
		if err != nil {
			return nil, toError(http.StatusBadRequest, err)
		}
		meta := h.GetJettonNormalizedMetadata(ctx, account.ID)
		if meta.Verification == core.TrustBlacklist {
			return &oas.GetChartRatesOK{}, nil
		}
		token = account.ID.ToRaw()
	}
	if params.Currency.Set {
		params.Currency.Value = strings.ToUpper(params.Currency.Value)
	}

	var startDate, endDate *int64
	if params.StartDate.Set {
		startDate = &params.StartDate.Value
	}
	if params.EndDate.Set {
		endDate = &params.EndDate.Value
	}

	var defaultPointsCount = 250
	if params.PointsCount.IsSet() {
		if params.PointsCount.Value > defaultPointsCount {
			return nil, toError(http.StatusBadRequest, fmt.Errorf("max points: %v", defaultPointsCount))
		}
		defaultPointsCount = params.PointsCount.Value
	}

	charts, err := h.ratesSource.GetRatesChart(token, params.Currency.Value, defaultPointsCount, startDate, endDate)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	points := make(oas.ChartPoints, 0, len(charts))
	for _, chart := range charts {
		points = append(points, []float64{float64(chart.Timestamp), chart.Price})
	}
	return &oas.GetChartRatesOK{Points: points}, nil
}

func (h *Handler) GetRates(ctx context.Context, params oas.GetRatesParams) (*oas.GetRatesOK, error) {
	if len(params.Tokens) == 0 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("tokens is required param"))
	}
	if len(params.Currencies) == 0 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("currencies is required param"))
	}
	if len(params.Tokens) > 100 || len(params.Currencies) > 100 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("max params limit is 100 items"))
	}

	human := false
	tokens := make([]string, 0, len(params.Tokens))
	for _, token := range params.Tokens {
		if decoded, err := url.QueryUnescape(token); err == nil {
			token = decoded
		}
		if len(token) == minTonAddressLength {
			human = true
		}
		if account, err := tongo.ParseAddress(token); err == nil {
			token = account.ID.ToRaw()
		} else {
			token = strings.ToUpper(token)
		}
		tokens = append(tokens, token)
	}

	currencies := params.Currencies
	for i := range currencies {
		if len(currencies[i]) == minTonAddressLength {
			human = true
		}
		if len(currencies[i]) < minTonAddressLength { // Not TON address
			currencies[i] = strings.ToUpper(currencies[i])
		}
	}

	todayRates, yesterdayRates, weekRates, monthRates, err := h.getRates()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}

	rates := make(map[string]oas.TokenRates)
	for _, token := range tokens {
		for _, currency := range currencies {
			rates, err = h.convertRates(ctx, rates, token, currency, todayRates, yesterdayRates, weekRates, monthRates)
			if err != nil {
				return nil, err
			}
		}
	}
	if human {
		temp := make(map[string]oas.TokenRates, len(rates))
		for k, v := range rates {
			if len(k) > minTonAddressLength {
				k = ton.MustParseAccountID(k).ToHuman(true, false)
			}
			temp[k] = v
		}
		rates = temp
	}

	return &oas.GetRatesOK{Rates: rates}, nil
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
	return tokenPrice / currencyPrice, nil
}

func (h *Handler) getRates() (todayRates, yesterdayRates, weekRates, monthRates map[string]float64, err error) {
	now := time.Now().UTC()
	timestamps := [4]int64{
		now.Unix(),
		now.AddDate(0, 0, -1).Unix(),
		now.AddDate(0, 0, -7).Unix(),
		now.AddDate(0, 0, -30).Unix(),
	}
	var results [4]map[string]float64
	for i, ts := range timestamps {
		results[i], err = h.ratesSource.GetRates(ts)
		if err != nil {
			return nil, nil, nil, nil, err
		}
	}
	return results[0], results[1], results[2], results[3], nil
}

func (h *Handler) convertRates(
	ctx context.Context,
	rates map[string]oas.TokenRates,
	token, currency string,
	todayRates, yesterdayRates, weekRates, monthRates map[string]float64,
) (map[string]oas.TokenRates, error) {
	trust := core.TrustNone
	if len(token) >= minTonAddressLength {
		accountID, err := ton.ParseAccountID(token)
		if err == nil {
			meta := h.GetJettonNormalizedMetadata(ctx, accountID)
			trust = meta.Verification
		}
	}

	todayCurrencyPrice, ok := todayRates[currency]
	if !ok {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("invalid currency: %v", currency))
	}

	rate, ok := rates[token]
	if !ok {
		rate = oas.TokenRates{
			Prices:  oas.NewOptTokenRatesPrices(oas.TokenRatesPrices{}),
			Diff24h: oas.NewOptTokenRatesDiff24h(oas.TokenRatesDiff24h{}),
			Diff7d:  oas.NewOptTokenRatesDiff7d(oas.TokenRatesDiff7d{}),
			Diff30d: oas.NewOptTokenRatesDiff30d(oas.TokenRatesDiff30d{}),
		}
	}

	tokenPrice := todayRates[token]
	if trust == core.TrustBlacklist {
		tokenPrice = 0
	}

	convertedTodayPrice := tokenPrice / todayCurrencyPrice
	rate.Prices.Value[currency] = convertedTodayPrice

	for _, entry := range []struct {
		hist  map[string]float64
		field map[string]string
	}{
		{yesterdayRates, rate.Diff24h.Value},
		{weekRates, rate.Diff7d.Value},
		{monthRates, rate.Diff30d.Value},
	} {
		diff := 0.0
		cp, _ := calculateConvertedPrice(entry.hist, currency, token)
		if token == "0:4a45235191e3f5fb68dc8394ff7ffff4b391e3fafadb7cd2cc432f1d9d9b3bc5" {
			h.logger.Info("[convertRates] currPrice", zap.Float64("price", convertedTodayPrice))
			h.logger.Info("[convertRates] prevPrice", zap.Float64("price", cp))
		}
		if cp != 0 && convertedTodayPrice != 0 {
			diff = 100 - math.Round(cp*10000/convertedTodayPrice)/100
		}
		switch {
		case diff < 0:
			entry.field[currency] = fmt.Sprintf("−%.2f%%", math.Abs(diff))
		case diff > 0:
			entry.field[currency] = fmt.Sprintf("+%.2f%%", diff)
		default:
			entry.field[currency] = "0.00%"
		}
	}

	rates[token] = rate
	return rates, nil
}
