package api

import (
	"testing"
)

func TestAlignConversionPrices(t *testing.T) {
	const (
		jetton = "0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe"
		now    = int64(1_756_000_000)
	)

	tests := []struct {
		name               string
		todayRates         map[string]float64
		todayTs            map[string]int64
		prevRates          map[string]float64
		prevTs             map[string]int64
		wantToken, wantCur float64
	}{
		{
			name:       "same generation: today prices used as is",
			todayRates: map[string]float64{jetton: 10, "TON": 1},
			todayTs:    map[string]int64{jetton: now, "TON": now},
			prevRates:  map[string]float64{jetton: 9, "TON": 1},
			prevTs:     map[string]int64{jetton: now - 60, "TON": now - 60},
			wantToken:  10, wantCur: 1,
		},
		{
			name:       "no timestamps: today prices used as is",
			todayRates: map[string]float64{jetton: 10, "TON": 1},
			todayTs:    map[string]int64{},
			prevRates:  map[string]float64{jetton: 9, "TON": 2},
			prevTs:     map[string]int64{},
			wantToken:  10, wantCur: 1,
		},
		{
			name: "stale token price: currency taken from previous snapshot",
			// the jetton price is from the 2-minute feed (now-120), the currency from
			// the 1-minute feed (now); the previous snapshot's currency (now-60... -120)
			// matches the jetton's generation better
			todayRates: map[string]float64{jetton: 10, "TON": 2},
			todayTs:    map[string]int64{jetton: now - 120, "TON": now},
			prevRates:  map[string]float64{jetton: 10, "TON": 1.8},
			prevTs:     map[string]int64{jetton: now - 120, "TON": now - 120},
			wantToken:  10, wantCur: 1.8,
		},
		{
			name:       "stale currency price: token taken from previous snapshot",
			todayRates: map[string]float64{jetton: 10, "TON": 2},
			todayTs:    map[string]int64{jetton: now, "TON": now - 120},
			prevRates:  map[string]float64{jetton: 9, "TON": 2},
			prevTs:     map[string]int64{jetton: now - 120, "TON": now - 120},
			wantToken:  9, wantCur: 2,
		},
		{
			name: "previous snapshot no closer: today prices kept",
			// prev snapshot is even further from the token's generation than today
			todayRates: map[string]float64{jetton: 10, "TON": 2},
			todayTs:    map[string]int64{jetton: now - 60, "TON": now},
			prevRates:  map[string]float64{jetton: 10, "TON": 1.8},
			prevTs:     map[string]int64{jetton: now - 240, "TON": now - 240},
			wantToken:  10, wantCur: 2,
		},
		{
			name: "gap above skew bound (slow fiat): today prices kept",
			// EUR was last written an hour ago; that is not feed staleness
			todayRates: map[string]float64{jetton: 10, "EUR": 0.5},
			todayTs:    map[string]int64{jetton: now, "EUR": now - 3600},
			prevRates:  map[string]float64{jetton: 9, "EUR": 0.5},
			prevTs:     map[string]int64{jetton: now - 60, "EUR": now - 3600},
			wantToken:  10, wantCur: 0.5,
		},
		{
			name:       "previous price is zero: today price kept",
			todayRates: map[string]float64{jetton: 10, "TON": 2},
			todayTs:    map[string]int64{jetton: now - 120, "TON": now},
			prevRates:  map[string]float64{jetton: 10, "TON": 0},
			prevTs:     map[string]int64{jetton: now - 120, "TON": now - 120},
			wantToken:  10, wantCur: 2,
		},
	}

	currencyOf := func(rates map[string]float64) string {
		if _, ok := rates["EUR"]; ok {
			return "EUR"
		}
		return "TON"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currency := currencyOf(tt.todayRates)
			gotToken, gotCur := alignConversionPrices(jetton, currency, tt.todayRates, tt.todayTs, tt.prevRates, tt.prevTs)
			if gotToken != tt.wantToken || gotCur != tt.wantCur {
				t.Fatalf("got token=%v currency=%v, want token=%v currency=%v", gotToken, gotCur, tt.wantToken, tt.wantCur)
			}
		})
	}
}
