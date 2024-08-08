package core

import (
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/ton"
)

type Multisig struct {
	AccountID ton.AccountID
	Seqno     int64
	Threshold int32
	Signers   []ton.AccountID
	Proposers []ton.AccountID
	Orders    []MultisigOrder
}

type MultisigOrder struct {
	AccountID        ton.AccountID
	OrderSeqno       int64
	Threshold        int32
	SentForExecution bool
	Signers          []ton.AccountID
	ApprovalsMask    []byte
	ApprovalsNum     int32
	ExpirationDate   int64
	Actions          []abi.MultisigSendMessageAction
}
