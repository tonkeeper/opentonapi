package rates

import (
	"math"
	"math/big"
)

const Pow128 = "340282366920938463463374607431768211456" // 2^128

func getInvariantForStableSwap(amp, x, y float64) float64 {
	// Validate input: reserves must be greater than zero
	if x <= 0 || y <= 0 {
		return 0
	}

	sum := x + y
	if sum == 0 {
		return 0
	}

	// Initialize previous and current invariant values
	invPrev := 0.0
	inv := sum

	// Use up to 255 iterations to reach convergence
	for i := 0; i <= 255; i++ {
		dP := inv

		dP = dP * inv / (x * 2) // use 'dp * ...' instead of '*=' because the first one seems more accurate
		dP = dP * inv / (y * 2)

		invPrev = inv
		firstTerm := amp*sum + dP*2
		secondTerm := (amp-1)*inv + dP*3

		// Prevent division by zero
		if secondTerm == 0 {
			return 0
		}

		inv = (inv * firstTerm) / secondTerm

		// Stop if converged
		if math.Abs(inv-invPrev) <= 1 {
			return inv
		}
	}

	// Reaching this part means not converge
	// It can be either if one of pool's reserve == 0 or if params looks like this: amp = 1, x = 1, y > 6e103

	return 0 // 0 means incorrect pool
}

func getOutTokensForStableSwap(amp, x, y, inv float64) float64 {
	// Input validation: amp, reserves, and invariant must be positive
	if amp <= 0 || x <= 0 || y <= 0 || inv <= 0 {
		return 0
	}

	sum := x
	pD := x * 2
	pD = (pD * (y * 2)) / inv
	sum += y

	// Prevent division by zero
	if pD == 0 {
		return 0
	}

	sum -= y
	D2 := inv * inv

	// Constants for iterative calculation
	c := (D2 / (amp * pD)) * y
	b := sum + (inv / amp)

	// Initial estimate of token balance
	prevTokenBalance := 0.0
	tokenBalance := (D2 + c) / (inv + b)

	for i := 0; i < 255; i++ {
		prevTokenBalance = tokenBalance

		denominator := (tokenBalance * 2) + b - inv
		if denominator == 0 {
			return 0
		}

		tokenBalance = ((tokenBalance * tokenBalance) + c) / denominator

		// Stop if converged
		if math.Abs(tokenBalance-prevTokenBalance) <= 1 {
			return tokenBalance
		}
	}

	// Reaching this part means not converge
	// It can be either if one of pool's reserve == 0 or if params looks like this: amp = 1, x = 1, y > 6e103

	return 0 // 0 means incorrect pool
}

func getOutTokensForWStableSwap(amp, x, dx, inv float64) float64 {
	// New x reserve amount
	s := x + dx

	// Prevent division by zero
	if s == 0 || amp == 0 {
		return 0
	}
	// Constants for iterative calculation
	c := (inv * inv) / (s * 2) * inv / (amp * 2)
	b := s + inv/amp

	y := inv
	prevY := y
	for i := 0; i <= 255; i++ {
		prevY = y
		y = (y*y + c) / (2*y + b - inv)

		// Stop if converged
		if math.Abs(y-prevY) <= 1 {
			return y
		}
	}

	// Reaching this part means not converge
	return 0
}

func calcSqrtP(sqrtP big.Float) float64 {
	pow128, ok := new(big.Float).SetString(Pow128) // 2^128
	if !ok {
		return 0
	}
	bigPrice := new(big.Float).SetPrec(1000).Quo(&sqrtP, pow128)    // (sqrtP >> 128)
	bigPrice = new(big.Float).SetPrec(1000).Mul(bigPrice, bigPrice) // bigPrice^2
	price, _ := bigPrice.Float64()
	return price
}
