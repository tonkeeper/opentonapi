package rates

import (
	"math"
	"math/big"
	"testing"
)

func TestGetInvariantForStableSwap(t *testing.T) {
	type Pool struct {
		name      string
		xReserves float64
		yReserves float64
		amp       float64
		// Expected output
		expectedInv float64
	}
	pools := []Pool{
		{
			name:        "get invariant for ordinary pool",
			xReserves:   4018398297881,
			yReserves:   4088833464707,
			amp:         100,
			expectedInv: 8107225762768.818,
		},
		{
			name:        "get invariant for bad balanced pool",
			xReserves:   345235553,
			yReserves:   8971637432040,
			amp:         100,
			expectedInv: 2048997240499.9214,
		},
		{
			name:        "get invariant for extra high amp pool",
			xReserves:   4018398297881,
			yReserves:   4088833464707,
			amp:         1e20,
			expectedInv: 8107231762588.0000,
		},
		{
			name:        "get invariant for low amp pool",
			xReserves:   4018398297881,
			yReserves:   4088833464707,
			amp:         1,
			expectedInv: 8107027778552.574219,
		},
		{
			name:        "get not converge invariant",
			xReserves:   1,
			yReserves:   6e103,
			amp:         1,
			expectedInv: 0,
		},
		{
			name:        "zero x reserves",
			xReserves:   0,
			yReserves:   1000,
			amp:         100,
			expectedInv: 0,
		},
		{
			name:        "zero y reserves",
			xReserves:   1000,
			yReserves:   0,
			amp:         100,
			expectedInv: 0,
		},
		{
			name:        "zero x and y reserves",
			xReserves:   0,
			yReserves:   0,
			amp:         100,
			expectedInv: 0,
		},
	}
	for _, p := range pools {
		t.Run(p.name, func(t *testing.T) {
			inv := getInvariantForStableSwap(p.amp, p.xReserves, p.yReserves)
			// diff between invariants must be <= 1e-12 (accuracy up to 12 decimals)
			if math.Abs(inv-p.expectedInv) > math.Max(math.Max(inv, p.expectedInv)*EPS, EPS) {
				t.Errorf("Incorrect invariant for (x: %v, y: %v, amp: %v) pool: got %v, want %v\n", p.xReserves, p.yReserves, p.amp, inv, p.expectedInv)
			}
		})
	}
}

func TestGetTokensOutForStableSwap(t *testing.T) {
	type Pool struct {
		name         string
		newXReserves float64
		yReserves    float64
		amp          float64
		inv          float64
		// Expected output
		expectedNewY float64
	}

	pools := []Pool{
		{
			name:         "get new Y reserve for ordinary pool",
			newXReserves: 4022416696178.880859,
			yReserves:    4088833464707,
			amp:          100,
			inv:          8107225762768.818359,
			expectedNewY: 4084813775070.736816,
		},
		{
			name:         "get new Y reserve for bad balanced pool",
			newXReserves: 345580788.553,
			yReserves:    8971637432040,
			amp:          100,
			inv:          2048997240499.921875,
			expectedNewY: 8967726023591.712891,
		},
		{
			name:         "get new Y reserve for extra high amp pool",
			newXReserves: 4022416696178.880859,
			yReserves:    4088833464707,
			amp:          1e20,
			inv:          8107231762588.0000,
			expectedNewY: 4084815066409.119141,
		},
		{
			name:         "get new Y reserve for low amp pool",
			newXReserves: 4022416696178.881,
			yReserves:    4088833464707,
			amp:          1,
			inv:          8107027778552.574219,
			expectedNewY: 4084770945830.244141,
		},
		{
			name:         "get not converge Y",
			newXReserves: 1.0001,
			yReserves:    1e10,
			amp:          1,
			inv:          6e80,
			expectedNewY: 0,
		},
		{
			name:         "zero x reserve",
			newXReserves: 0,
			yReserves:    1000,
			amp:          100,
			inv:          1000,
			expectedNewY: 0,
		},
		{
			name:         "zero y reserve",
			newXReserves: 1000,
			yReserves:    0,
			amp:          100,
			inv:          1000,
			expectedNewY: 0,
		},
		{
			name:         "zero amp",
			newXReserves: 1000,
			yReserves:    1000,
			amp:          0,
			inv:          1000,
			expectedNewY: 0,
		},
		{
			name:         "zero invariant",
			newXReserves: 1000,
			yReserves:    1000,
			amp:          100,
			inv:          0,
			expectedNewY: 0,
		},
	}
	for _, p := range pools {
		t.Run(p.name, func(t *testing.T) {
			newY := getOutTokensForStableSwap(p.amp, p.newXReserves, p.yReserves, p.inv)
			// diff between y must be <= 1e-12 (accuracy up to 12 decimals)
			if math.Abs(newY-p.expectedNewY) > math.Max(math.Max(newY, p.expectedNewY)*EPS, EPS) {
				t.Errorf("Incorrect new Y reserve for (x: %v, y: %v, amp: %v, inv: %v) pool: got %v, want %v\n", p.newXReserves, p.yReserves, p.amp, p.inv, newY, p.expectedNewY)
			}
		})
	}
}

func TestCalcSqrtP(t *testing.T) {
	type Pool struct {
		name  string
		sqrtP big.Float
		// Expected output
		expectedPrice float64
	}

	sqrtP1, _ := new(big.Float).SetString("47762678216590087718128169122709613537")
	sqrtP2, _ := new(big.Float).SetString("5953309917844247849267535390012688668899")
	pools := []Pool{
		{
			name:          "get pool with equals decimals",
			sqrtP:         *sqrtP1,
			expectedPrice: 0.019701461865378845,
		},
		{
			name:          "get pool with different decimals",
			sqrtP:         *sqrtP2,
			expectedPrice: 306.0822134857971,
		},
	}
	for _, p := range pools {
		t.Run(p.name, func(t *testing.T) {
			price := calcSqrtP(p.sqrtP)
			// diff between y must be <= 1e-12 (accuracy up to 12 decimals)
			if math.Abs(price-p.expectedPrice) > math.Max(math.Max(price, p.expectedPrice)*EPS, EPS) {
				t.Errorf("Incorrect price for sqrtP pool: got %v, want %v\n", price, p.expectedPrice)
			}
		})
	}
}
