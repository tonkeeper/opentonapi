package model

import (
	"encoding/json"
	"math/big"
)

// NTon represents a value in Nano TON (1 TON = 1e9 NTon).
// It wraps *big.Int and serializes to JSON as a quoted string
// to preserve precision for consumers.
type NTon struct{ *big.Int }

func (n NTon) MarshalJSON() ([]byte, error) {
	if n.Int == nil {
		return []byte("null"), nil
	}
	return json.Marshal(n.Int.String())
}

func (n NTon) String() string {
	if n.Int == nil {
		return "null"
	}
	return n.Int.String()
}
