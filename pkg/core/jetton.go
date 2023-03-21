package core

import (
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/tongo"
)

type JettonWallet struct {
	Address       tongo.AccountID
	Balance       decimal.Decimal
	OwnerAddress  *tongo.AccountID
	JettonAddress tongo.AccountID
	Code          []byte
}
