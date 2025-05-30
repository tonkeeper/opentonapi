package rates

import (
	"math"
)

func getInvariantForStableSwap(amp, x, y float64) float64 {
	sum := x + y
	if sum == 0 {
		return 0
	}

	invPrev := float64(0) // prev invariant
	inv := sum            // invariant

	for range 255 {
		dP := inv

		dP *= inv / (x * 2)
		dP *= inv / (y * 2)

		invPrev = inv
		firstTerm := amp*sum + dP*2
		secondTerm := (amp-1)*inv + dP*3

		inv *= firstTerm / secondTerm

		if math.Abs(inv-invPrev) <= 1 {
			return inv
		}
	}

	return 0
}

func getOutTokensForStableSwap(amp, x, y, inv float64) float64 {
	sum := x
	pD := x * 2
	pD *= (y * 2) / inv
	sum += y

	sum -= y
	D2 := inv * inv

	c := (D2 / (amp * pD)) * y
	b := sum + (inv / amp)

	prevTokenBalance := 0.0
	tokenBalance := (D2 + c) / (inv + b)
	for range 255 {
		prevTokenBalance = tokenBalance

		tokenBalance = ((tokenBalance * tokenBalance) + c) / ((tokenBalance * 2) + b - inv)

		if math.Abs(tokenBalance-prevTokenBalance) <= 1 {
			return tokenBalance
		}
	}

	return 0
}
