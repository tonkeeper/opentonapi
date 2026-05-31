package core

import (
	"math/big"

	"github.com/tonkeeper/tongo"
)

type CurrencyType string

const (
	CurrencyNative CurrencyType = "native"
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

type VaultDepositInfo struct {
	Price Price
	Vault tongo.AccountID
}

func PriceNanoGram(amount int64) Price {
	return Price{
		Currency: Currency{Type: CurrencyNative},
		Amount:   *big.NewInt(amount),
	}
}

func NewCurrencyJetton(master *tongo.AccountID) Currency {
	return Currency{
		Type:   CurrencyJetton,
		Jetton: master,
	}
}
