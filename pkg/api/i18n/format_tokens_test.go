package i18n

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatTONs(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		want   string
	}{
		{
			amount: 0,
			want:   "0 TON",
		},
		{
			amount: -1_000_000_000,
			want:   "-1 TON",
		},
		{
			amount: 33000_144_000_000,
			want:   "33 000 TON",
		},
		{
			amount: 33000_544_000_000,
			want:   "33 000 TON",
		},
		{
			amount: 33000_944_000_000,
			want:   "33 000 TON",
		},
		{
			amount: 1_249_000_000,
			want:   "1.24 TON",
		},
		{
			amount: 1_241_000_000,
			want:   "1.24 TON",
		},
		{
			amount: 143_945,
			want:   "0.000143 TON",
		},
		{
			amount: 143_145,
			want:   "0.000143 TON",
		},
		{
			amount: -33000_000_000_000,
			want:   "-33 000 TON",
		},
		{
			amount: 1_000_000_000,
			want:   "1 TON",
		},
		{
			amount: 1_000_000,
			want:   "0.001 TON",
		},
		{
			amount: 566_450_533_222_111,
			want:   "566 450 TON",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTONs(tt.amount)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFormatJettons(t *testing.T) {
	tests := []struct {
		name    string
		decimal int32
		symbol  string
		amount  int64
		want    string
	}{
		{
			amount:  0,
			decimal: 6,
			symbol:  "USDT",
			want:    "0 USDT",
		},
		{
			amount:  33000_144_000_000,
			decimal: 6,
			symbol:  "USDT",
			want:    "33 000 144 USDT",
		},
		{
			amount:  566_450_533_222_111,
			decimal: 6,
			symbol:  "USDT",
			want:    "566 450 533 USDT",
		},
		{
			amount:  1_566_450_533_222_111,
			decimal: 6,
			symbol:  "USDT",
			want:    "1 566 450 533 USDT",
		},
		{
			amount:  143_145,
			decimal: 6,
			symbol:  "USDT",
			want:    "0.143 USDT",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTokens(*big.NewInt(tt.amount), tt.decimal, tt.symbol)
			require.Equal(t, tt.want, got)
		})
	}
}
