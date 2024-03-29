package i18n

import (
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
			amount: 33000_544_000_000,
			want:   "33,000.544 TON",
		},
		{
			amount: 33000_000_000_000,
			want:   "33,000 TON",
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
			amount: 566_450_333_222_111,
			want:   "566,450.333222111 TON",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTONs(tt.amount)
			require.Equal(t, tt.want, got)
		})
	}
}
