package defi

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestJettonPositionNanoTON(t *testing.T) {
	rates := map[string]float64{
		"TON":        5.0,
		"0:aabbccdd": 2.5,
	}

	tests := []struct {
		name     string
		balance  string
		decimals int
		want     int64
	}{
		{
			name:     "whole token",
			balance:  "1000000000",
			decimals: 9,
			want:     500_000_000,
		},
		{
			name:     "two tokens",
			balance:  "2000000000",
			decimals: 9,
			want:     1_000_000_000,
		},
		{
			name:     "zero balance",
			balance:  "0",
			decimals: 9,
			want:     0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bal, err := decimal.NewFromString(tt.balance)
			require.NoError(t, err)
			got := JettonPositionNanoTON(rates, "0:aabbccdd", bal, tt.decimals)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestJettonPositionNanoTONMissingRate(t *testing.T) {
	rates := map[string]float64{"TON": 5.0}
	bal, _ := decimal.NewFromString("1000000000")
	require.Equal(t, int64(0), JettonPositionNanoTON(rates, "0:unknown", bal, 9))
}

func TestJettonPositionNanoTONMissingTON(t *testing.T) {
	rates := map[string]float64{"0:aabbccdd": 2.5}
	bal, _ := decimal.NewFromString("1000000000")
	require.Equal(t, int64(0), JettonPositionNanoTON(rates, "0:aabbccdd", bal, 9))
}
