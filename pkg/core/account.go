package core

import (
	"math/big"

	"github.com/tonkeeper/tongo/abi"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
)

// Account holds low-level details about a particular account taken directly from the blockchain.
type Account struct {
	AccountAddress    tongo.AccountID
	Status            tlb.AccountStatus
	TonBalance        int64
	ExtraBalances     map[uint32]decimal.Decimal
	LastTransactionLt uint64
	Code              []byte
	Data              []byte
	FrozenHash        *tongo.Bits256
	Storage           StorageInfo
	Interfaces        []abi.ContractInterface
	LastActivityTime  int64
	GetMethods        []string
	Libraries         map[tongo.Bits256]*SimpleLib
}

// StorageInfo is taken from TLB storage_stat:StorageInfo.
type StorageInfo struct {
	UsedCells       big.Int
	UsedBits        big.Int
	UsedPublicCells big.Int
	LastPaid        uint32
	DuePayment      int64
}

// AccountInfo extends Account type to hold additional human-friendly information about a particular account.
type AccountInfo struct {
	Account      Account
	MemoRequired *bool
	Name         *string
	Icon         *string
	IsScam       *bool
}

// Contract represents an account but contains a few fields that are only relevant for smart contracts.
type Contract struct {
	Status    tlb.AccountStatus
	Code      []byte
	Data      []byte
	Libraries map[tongo.Bits256]*SimpleLib
}
