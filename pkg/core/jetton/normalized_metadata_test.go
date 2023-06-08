package jetton

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo/tlb"
)

func TestScale(t *testing.T) {
	tests := []struct {
		name     string
		amount   *big.Int
		decimals int
		want     string
	}{
		{
			name:     "decimals 0",
			amount:   big.NewInt(100),
			decimals: 0,
			want:     "100",
		},
		{
			name:     "decimals 1",
			amount:   big.NewInt(100),
			decimals: 1,
			want:     "10",
		},
		{
			name:     "decimals 2",
			amount:   big.NewInt(100),
			decimals: 2,
			want:     "1",
		},
		{
			name:     "decimals 3",
			amount:   big.NewInt(100),
			decimals: 3,
			want:     "0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlbAmount := tlb.VarUInteger16(*tt.amount)
			amount := Scale(tlbAmount, tt.decimals)
			require.Equal(t, tt.want, amount.String())
		})
	}
}
