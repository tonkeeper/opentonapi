package model

import (
	"encoding/json"
	"math/big"
)

// NanoGram represents a value in Nano TON (1 TON = 1e9 NanoGram).
// It wraps *big.Int and serializes to JSON as a quoted string
// to preserve precision for consumers.
type NanoGram struct{ *big.Int }

func (n NanoGram) MarshalJSON() ([]byte, error) {
	if n.Int == nil {
		return []byte("null"), nil
	}
	return json.Marshal(n.Int.String())
}

func (n NanoGram) String() string {
	if n.Int == nil {
		return "null"
	}
	return n.Int.String()
}
