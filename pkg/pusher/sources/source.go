package sources

import (
	"context"

	"github.com/tonkeeper/tongo"
)

type SubscribeToTransactionsOptions struct {
	Accounts      []tongo.AccountID
	AllAccounts   bool
	Operations    []string
	AllOperations bool
}

// SubscribeToMempoolOptions configures subscription to mempool events.
type SubscribeToMempoolOptions struct {
	// Emulation if set, opentonapi will send a message payload and additionally a list of accounts
	// that are involved in the corresponding trace.
	Accounts []tongo.AccountID
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

// EmulationMessageEventData represents a notification about a new pending inbound message.
// After opentonapi receives a message, it emulates what happens when the message lands on the blockchain.
// Then it sends the message and the emulation results to subscribers.
// This is part of our API contract with subscribers.
type EmulationMessageEventData struct {
	BOC []byte `json:"boc"`
	// InvolvedAccounts is a list of accounts that are involved in the corresponding trace of the message.
	// The trace is a result of emulation.
	InvolvedAccounts []tongo.AccountID `json:"involved_accounts"`
}

// MemPoolSource provides a method to subscribe to notifications about pending inbound messages.
type MemPoolSource interface {
	SubscribeToMessages(ctx context.Context, deliveryFn DeliveryFn, opts SubscribeToMempoolOptions) (CancelFn, error)
}
