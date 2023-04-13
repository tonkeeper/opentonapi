package sources

import "github.com/tonkeeper/tongo"

type SubscribeToTransactionsOptions struct {
	AllAccounts bool
	Accounts    []tongo.AccountID
}

// DeliveryTxFn describes a callback that will be triggered once a new transaction happens.
type DeliveryTxFn func(eventData []byte)

// CancelFn has to be called to unsubscribe.
type CancelFn func()

// TransactionSource provides a method to subscribe to notifications about new transactions from the blockchain.
type TransactionSource interface {
	SubscribeToTransactions(deliveryFn DeliveryTxFn, opts SubscribeToTransactionsOptions) CancelFn
}
