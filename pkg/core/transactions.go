package core

import (
	"encoding/json"
	"fmt"
	"github.com/tonkeeper/tongo/ton"
	"math/big"
	"reflect"
	"strings"

	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
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
	ResultCode     int32
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
	Utime      int64
	InMsg      *Message
	OutMsgs    []Message
	BlockID    tongo.BlockID
	OrigStatus tlb.AccountStatus
	EndStatus  tlb.AccountStatus

	EndBalance int64

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
	TotalFee int64
	Raw      []byte
}

type MessageID struct {
	CreatedLt   uint64
	Source      *tongo.AccountID
	Destination *tongo.AccountID
}

func (m MessageID) String() string {
	builder := strings.Builder{}
	builder.Write([]byte(fmt.Sprintf("%d/", m.CreatedLt)))
	if m.Source != nil {
		builder.Write([]byte(m.Source.ToRaw()))
	} else {
		builder.Write([]byte("x"))
	}
	if m.Destination != nil {
		builder.Write([]byte("/" + m.Destination.ToRaw()))
	} else {
		builder.Write([]byte("/x"))
	}
	return builder.String()
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
	Hash              ton.Bits256
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

func (m *Message) UnmarshalJSON(data []byte) error {
	type Alias Message
	r := struct {
		*Alias
		DecodedBody struct {
			Operation string
			Value     json.RawMessage
		}
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	var msgTypes map[string]any
	switch r.MsgType {
	case IntMsg:
		msgTypes = abi.KnownMsgInTypes
	case ExtOutMsg:
		msgTypes = abi.KnownMsgExtOutTypes
	case ExtInMsg:
		msgTypes = abi.KnownMsgExtInTypes
	}
	if v, present := msgTypes[r.DecodedBody.Operation]; present {
		p := reflect.New(reflect.TypeOf(v))
		if err := json.Unmarshal(r.DecodedBody.Value, p.Interface()); err != nil {
			return err
		}
		m.DecodedBody = &DecodedMessageBody{
			Operation: r.DecodedBody.Operation,
			Value:     p.Elem().Interface(),
		}
		return nil
	}
	return nil
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
