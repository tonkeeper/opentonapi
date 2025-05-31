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

		dP = dP * inv / (x * 2) // use 'dp * ...' instead of '*=' because the first one seems more accurate
		dP = dP * inv / (y * 2)

		invPrev = inv
		firstTerm := amp*sum + dP*2
		secondTerm := (amp-1)*inv + dP*3

		inv = (inv * firstTerm) / secondTerm

		if math.Abs(inv-invPrev) <= 1 {
			return inv
		}
	}

	// Reaching this part means not converge
	// There params can cause it: amp = 1, x = 1, y > 6e103
	// But it's actually impossible

	return 0 // 0 means incorrect pool
}

func getOutTokensForStableSwap(amp, x, y, inv float64) float64 {
	sum := x
	pD := x * 2
	pD = (pD * (y * 2)) / inv
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

	// Reaching this part means not converge
	// There params can cause it: amp = 1, x = 1, y > 6e103
	// But it's actually impossible

	return 0 // 0 means incorrect pool
}
