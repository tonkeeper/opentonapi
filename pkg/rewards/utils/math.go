package utils

import "math/big"

func MulDiv(a, b, c *big.Int) *big.Int {
	return new(big.Int).Div(new(big.Int).Mul(a, b), c)
}

func InaccurateDivFloat(a, b *big.Int) float64 {
	div, _ := new(big.Float).Quo(new(big.Float).SetInt(a), new(big.Float).SetInt(b)).Float64()
	return div
}
