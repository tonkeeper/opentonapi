package core

import (
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/tongo/ton"
)

type InscriptionBalance struct {
	Account ton.AccountID
	Amount  decimal.Decimal
	Ticker  string
}

// InscriptionMessage describes a message according to the TON-20 specification: https://docs.tonano.io/introduction/overview#indexer.
type InscriptionMessage struct {
	// Hash of the TON blockchain message.
	Hash      ton.Bits256
	Success   bool
	Operation string
	Ticker    string
	Amount    uint64
	Source    ton.AccountID
	Dest      *ton.AccountID
}
