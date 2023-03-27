package core

import (
	"github.com/tonkeeper/tongo/tlb"
	"math/big"

	"github.com/tonkeeper/tongo"
)

type TransactionID struct {
	Hash    tongo.Bits256
	Lt      uint64
	Account tongo.AccountID
}

// TransactionType stands for transaction kind from [4.2.4] from the TON whitepaper.
type TransactionType string

const (
	OrdinaryTx     TransactionType = "TransOrd"
	TickTockTx     TransactionType = "TransTickTock"
	SplitPrepareTx TransactionType = "TransSplitPrepare"
	SplitInstallTx TransactionType = "TransSplitInstall"
	MergePrepareTx TransactionType = "TransMergePrepare"
	MergeInstallTx TransactionType = "TransMergeInstall"
	StorageTx      TransactionType = "TransStorage"
)

type StateUpdate struct {
	OldHash tongo.Bits256
	NewHash tongo.Bits256
}

type TxComputeSkipReason = tlb.ComputeSkipReason
type TxAccStatusChange = tlb.AccStatusChange

type TxComputePhase struct {
	Skipped    bool
	SkipReason TxComputeSkipReason
	Success    bool
	GasFees    uint64
	GasUsed    big.Int
	VmSteps    uint32
	ExitCode   int32
}

type TxStoragePhase struct {
	StorageFeesCollected uint64
	StorageFeesDue       *uint64
	StatusChange         TxAccStatusChange
}

type TxCreditPhase struct {
	DueFeesCollected uint64
	CreditGrams      uint64
}

type TxActionPhase struct {
	Success        bool
	TotalActions   uint16
	SkippedActions uint16
	FwdFees        uint64
	TotalFees      uint64
}

type TxBouncePhase struct {
	Type BouncePhaseType
}

type BouncePhaseType string

const (
	BounceNegFunds BouncePhaseType = "TrPhaseBounceNegfunds"
	BounceNoFunds  BouncePhaseType = "TrPhaseBounceNofunds "
	BounceOk       BouncePhaseType = "TrPhaseBounceOk"
)

type Transaction struct {
	TransactionID
	Type       TransactionType
	Success    bool
	Fee        int64
	OtherFee   int64
	StorageFee int64
	Utime      int64
	InMsg      *Message
	OutMsgs    []Message
	Data       []byte
	BlockID    tongo.BlockID
	OrigStatus tlb.AccountStatus
	EndStatus  tlb.AccountStatus

	PrevTransHash tongo.Bits256
	PrevTransLt   uint64

	StateHashUpdate tlb.HashUpdate
	ComputePhase    *TxComputePhase
	StoragePhase    *TxStoragePhase
	CreditPhase     *TxCreditPhase
	ActionPhase     *TxActionPhase
	BouncePhase     *TxBouncePhase

	Aborted   bool
	Destroyed bool
}

type MessageID struct {
	CreatedLt   uint64
	Source      *tongo.AccountID
	Destination *tongo.AccountID
}

func (m MessageID) IsExternal() bool {
	return m.Source == nil
}

func (m Message) IsEmission() bool {
	return m.Source != nil && m.Source.IsZero() && m.Source.Workchain == -1 && m.Bounced == false
}

type Message struct {
	MessageID
	IhrDisabled bool
	Bounce      bool
	Bounced     bool
	Value       int64
	FwdFee      int64
	IhrFee      int64
	ImportFee   int64
	Init        []byte
	Body        []byte
	CreatedAt   uint32
	// OpCode is the first 32 bits of a message body indicating a possible operation.
	OpCode      *uint32
	DecodedBody *DecodedMessageBody
}

// DecodedMessageBody contains a message body decoded by tongo.abi package.
type DecodedMessageBody struct {
	Operation string
	Value     any
}
