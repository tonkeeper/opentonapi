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
