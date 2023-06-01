package core

import (
	"math/big"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

// Account holds low-level details about a particular account taken directly from the blockchain.
type Account struct {
	AccountAddress    tongo.AccountID
	Status            string
	TonBalance        int64
	ExtraBalances     map[uint32]decimal.Decimal
	LastTransactionLt uint64
	Code              []byte
	Data              []byte
	FrozenHash        *tongo.Bits256
	Storage           StorageInfo
	Interfaces        []string
	LastActivityTime  int64
	GetMethods        []string
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

// HasAnyIntefaceImplemented returns true if this account implements at least one of the given interfaces.
func (acc *Account) HasAnyIntefaceImplemented(interfaces ...abi.ContractInterface) bool {
	for _, implemented := range acc.Interfaces {
		for _, iface := range interfaces {
			if implemented == string(iface) {
				return true
			}
		}
	}
	return false
}
