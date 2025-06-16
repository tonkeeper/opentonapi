package core

import (
	"github.com/tonkeeper/tongo"
	"math/big"
)

type CurrencyType string

const (
	CurrencyTON    CurrencyType = "native"
	CurrencyExtra  CurrencyType = "extra_currency"
	CurrencyJetton CurrencyType = "jetton"
	CurrencyFiat   CurrencyType = "fiat"
)

type Price struct {
	Type       CurrencyType
	Amount     big.Int
	Jetton     *tongo.AccountID
	CurrencyID *int32
}
