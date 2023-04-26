package sources

import (
	"context"

	"github.com/tonkeeper/tongo"
)

type SubscribeToTransactionsOptions struct {
	AllAccounts bool
	Accounts    []tongo.AccountID
}

// DeliveryFn describes a callback that will be triggered once an event happens.
type DeliveryFn func(eventData []byte)

// CancelFn has to be called to unsubscribe.
type CancelFn func()

// TransactionEventData represents a notification about a new transaction.
// This is part of our API contract with subscribers.
type TransactionEventData struct {
	AccountID tongo.AccountID `json:"account_id"`
	Lt        uint64          `json:"lt"`
	TxHash    string          `json:"tx_hash"`
}

// TransactionSource provides a method to subscribe to notifications about new transactions from the blockchain.
type TransactionSource interface {
	SubscribeToTransactions(ctx context.Context, deliveryFn DeliveryFn, opts SubscribeToTransactionsOptions) CancelFn
}

// MessageEventData represents a notification about a new pending inbound message.
// This is part of our API contract with subscribers.
type MessageEventData struct {
	BOC []byte `json:"boc"`
}

// MemPoolSource provides a method to subscribe to notifications about pending inbound messages.
type MemPoolSource interface {
	SubscribeToMessages(ctx context.Context, deliveryFn DeliveryFn) (CancelFn, error)
}
