package core

import (
	"github.com/tonkeeper/tongo/boc"

	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"

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
	Skipped          bool
	SkipReason       TxComputeSkipReason
	Success          bool
	MsgStateUsed     bool
	AccountActivated bool
	GasFees          uint64
	GasUsed          uint64
	GasLimit         uint64
	GasCredit        uint64
	Mode             int8
	ExitCode         int32
	ExitArg          int32
	VmSteps          uint32
}

type TxStoragePhase struct {
	StorageFeesCollected uint64
	StorageFeesDue       *uint64
	StatusChange         TxAccStatusChange
}

type TxCreditPhase struct {
	DueFeesCollected uint64
	CreditGrams      uint64
	CreditExtra      ExtraCurrency
}

type TxActionPhase struct {
	Success         bool
	Valid           bool
	NoFunds         bool
	StatusChange    TxAccStatusChange
	ResultCode      int32
	ResultArg       int32
	TotalActions    uint16
	SpecActions     uint16
	SkippedActions  uint16
	TotalFwdFees    uint64
	TotalActionFees uint64
	MsgsCreated     uint16
	TotMsgSize      StorageUsedShort
}

type StorageUsedShort struct {
	Cells int64
	Bits  int64
}

type TxBouncePhase struct {
	Type       BouncePhaseType
	MsgSize    *StorageUsedShort // BounceNofunds + BounceOk
	ReqFwdFees *uint64           // BounceNofunds
	MsgFees    *uint64           // BounceOk
	FwdFees    *uint64           // BounceOk
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
	Utime      int64
	InMsg      *Message
	OutMsgs    []Message
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

	// StorageFee collected during the Storage Phase.
	StorageFee int64
	// TotalFee is the original total_fee of a transaction directly from the blockchain.
	TotalFee      int64
	TotalFeeExtra ExtraCurrency
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

type MsgType string

const (
	IntMsg    MsgType = "IntMsg"
	ExtInMsg  MsgType = "ExtInMsg"
	ExtOutMsg MsgType = "ExtOutMsg"
)

type Message struct {
	MessageID
	MsgType           MsgType
	SourceExtern      *ExternalAddress
	DestinationExtern *ExternalAddress
	IhrDisabled       bool
	Bounce            bool
	Bounced           bool
	Value             int64
	FwdFee            int64
	IhrFee            int64
	ImportFee         int64
	Init              []byte
	InitInterfaces    []abi.ContractInterface
	Body              []byte
	CreatedAt         uint32
	// OpCode is the first 32 bits of a message body indicating a possible operation.
	OpCode      *uint32
	DecodedBody *DecodedMessageBody
}

// DecodedMessageBody contains a message body decoded by tongo.abi package.
type DecodedMessageBody struct {
	Operation string
	Value     any
}

// ExternalAddress represents either the source or destination address of
// external inbound(ExtInMsg) or external outbound(ExtOutMsg) message correspondingly.
type ExternalAddress = boc.BitString

func externalAddressFromTlb(address tlb.MsgAddress) *ExternalAddress {
	if address.SumType != "AddrExtern" {
		return nil
	}
	external := address.AddrExtern.ExternalAddress
	return &external
}
