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
	Lock          *JettonWalletLockData
	Extensions    []string
}

type JettonHolder struct {
	JettonAddress tongo.AccountID
	Address       tongo.AccountID
	Owner         tongo.AccountID
	Balance       decimal.Decimal
}

type JettonMaster struct {
	// Address of a jetton master.
	Address     tongo.AccountID
	TotalSupply big.Int
	Mintable    bool
	Admin       *tongo.AccountID
}

type JettonWalletLockData struct {
	FullBalance decimal.Decimal
	UnlockTime  int64
}

type JettonOperationType = string

const (
	TransferJettonOperation JettonOperationType = "transfer"
	MintJettonOperation     JettonOperationType = "mint"
	BurnJettonOperation     JettonOperationType = "burn"
	UnknownJettonOperation  JettonOperationType = "unknown"
)

type JettonOperation struct {
	Operation      JettonOperationType
	Source         *tongo.AccountID
	Destination    *tongo.AccountID
	JettonMaster   tongo.AccountID
	TraceID        TraceID
	DestEndBalance decimal.Decimal
	Amount         decimal.Decimal
	QueryID        uint64
	ForwardPayload string
	Lt             uint64
	Utime          int64
}
