package sources

import (
	"context"
	"fmt"

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

// SubscribeToBlockHeadersOptions configures subscription to block events.
type SubscribeToBlockHeadersOptions struct {
	// Workchain, if set, opentonapi will filter out blocks that are not from the specified workchain.
	Workchain *int `json:"workchain,omitempty"`
}

// BlockEvent represents a notification about a new block.
// This is part of our API contract with subscribers.
type BlockEvent struct {
	Workchain int32  `json:"workchain"`
	Shard     string `json:"shard"`
	Seqno     uint32 `json:"seqno"`
	RootHash  string `json:"root_hash"`
	FileHash  string `json:"file_hash"`
}

func (e BlockEvent) String() string {
	return fmt.Sprintf("(%d,%x,%d)", e.Workchain, e.Shard, e.Seqno)
}

// BlockHeadersSource provides a method to subscribe to notifications about new blocks in the TON network.
type BlockHeadersSource interface {
	SubscribeToBlockHeaders(ctx context.Context, deliveryFn DeliveryFn, opts SubscribeToBlockHeadersOptions) CancelFn
}

type SubscribeToBlocksOptions struct {
	MasterchainSeqno uint32 `json:"masterchain_seqno,omitempty"`
	// RateLimit defines the rate limit (KB/sec) for the block streaming.
	RateLimit int
}

type Block struct {
	Workchain int32  `json:"workchain"`
	Shard     string `json:"shard"`
	Seqno     uint32 `json:"seqno"`
	RootHash  string `json:"root_hash"`
	FileHash  string `json:"file_hash"`
	Raw       []byte `json:"raw"`
}

// BlockchainSliceEvent represents a notification about a new bunch of blocks in the blockchain.
type BlockchainSliceEvent struct {
	MasterchainSeqno uint32 `json:"masterchain_seqno"`
	// Blocks contains one masterchain block and all blocks from the basechain created since the previous blockchain slice.
	Blocks []Block `json:"blocks"`
}

type BlockSource interface {
	SubscribeToBlocks(ctx context.Context, deliveryFn DeliveryFn, opts SubscribeToBlocksOptions) (CancelFn, error)
}

// TraceEventData represents a notification about a completed trace.
// This is part of our API contract with subscribers.
type TraceEventData struct {
	AccountIDs []tongo.AccountID `json:"accounts"`
	Hash       string            `json:"hash"`
}
