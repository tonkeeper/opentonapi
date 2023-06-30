package core

import (
	"math/big"

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

type JettonMaster struct {
	// Address of a jetton master.
	Address     tongo.AccountID
	TotalSupply big.Int
	Mintable    bool
}
