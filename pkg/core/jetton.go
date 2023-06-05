package core

import (
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/tongo"
)

type JettonWallet struct {
	// Address of a jetton wallet.
	Address      tongo.AccountID
	Balance      decimal.Decimal
	OwnerAddress *tongo.AccountID
	// JettonAddress of a jetton master.
	JettonAddress tongo.AccountID
	Code          []byte
}

const UnknownJettonName = "UKWN"
