package core

import (
	"math/big"

	"github.com/tonkeeper/tongo"
)

type Account struct {
	AccountAddress    tongo.AccountID
	Status            string
	TonBalance        int64
	ExtraBalances     map[uint32]big.Int
	LastTransactionLt uint64
	Code              []byte
	Data              []byte
	FrozenHash        *tongo.Bits256
	Storage           StorageInfo
}

// StorageInfo is taken from TLB storage_stat:StorageInfo.
type StorageInfo struct {
	UsedCells       big.Int
	UsedBits        big.Int
	UsedPublicCells big.Int
	LastPaid        uint32
	DuePayment      int64
}
