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

type Currency struct {
	Type       CurrencyType
	Jetton     *tongo.AccountID
	CurrencyID *int32
}

type Price struct {
	Currency Currency
	Amount   big.Int
}
