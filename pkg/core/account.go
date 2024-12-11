package core

import (
	"math/big"

	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/ton"

	"github.com/tonkeeper/tongo/tlb"
)

// Account holds low-level details about a particular account taken directly from the blockchain.
type Account struct {
	AccountAddress      ton.AccountID
	Status              tlb.AccountStatus
	TonBalance          int64
	ExtraBalances       ExtraCurrencies
	LastTransactionLt   uint64
	LastTransactionHash ton.Bits256
	Code                []byte
	Data                []byte
	FrozenHash          *ton.Bits256
	Storage             StorageInfo
	Interfaces          []abi.ContractInterface
	LastActivityTime    int64
	GetMethods          []string
	Libraries           map[ton.Bits256]*SimpleLib
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
	Status            tlb.AccountStatus
	Balance           int64
	Code              []byte
	Data              []byte
	Libraries         map[ton.Bits256]*SimpleLib
	LastTransactionLt uint64
}

type AccountStat struct {
	TonBalance    int64
	AccountID     ton.AccountID
	NftsCount     int32
	JettonsCount  int32
	MultisigCount int32
	StakingCount  int32
}
