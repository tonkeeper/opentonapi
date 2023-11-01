package tonapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sse "github.com/r3labs/sse/v2"
	"github.com/tonkeeper/tongo"
)

// MempoolEventData represents the data part of a new-pending-message event.
type MempoolEventData struct {
	BOC []byte `json:"boc"`
}

// TransactionEventData represents the data part of a new-transaction event.
type TransactionEventData struct {
	AccountID tongo.AccountID `json:"account_id"`
	Lt        uint64          `json:"lt"`
	TxHash    string          `json:"tx_hash"`
}

// TraceEventData represents a notification about a completed trace.
type TraceEventData struct {
	AccountIDs []tongo.AccountID `json:"accounts"`
	Hash       string            `json:"hash"`
}

// TraceHandler is a callback that handles a new trace event.
type TraceHandler func(TraceEventData)

// TransactionHandler is a callback that handles a new transaction event.
type TransactionHandler func(data TransactionEventData)

// MempoolHandler is a callback that handles a new mempool event.
type MempoolHandler func(data MempoolEventData)

// StreamingAPI provides a convenient way to receive events happening on the TON blockchain.
type StreamingAPI struct {
	logger   Logger
	apiKey   string
	endpoint string
}

type StreamingOptions struct {
	logger   Logger
	apiKey   string
	endpoint string
}

type StreamingOption func(*StreamingOptions)

// WithStreamingEndpoint configures a StreamingAPI instance to use the specified endpoint instead of https://tonapi.io.
func WithStreamingEndpoint(endpoint string) StreamingOption {
	return func(o *StreamingOptions) {
		o.endpoint = endpoint
	}
}

// WithStreamingTestnet configures a StreamingAPI instance to use an endpoint to work with testnet.
func WithStreamingTestnet() StreamingOption {
	return func(o *StreamingOptions) {
		o.endpoint = TestnetTonApiURL
	}
}

// WithStreamingToken configures a StreamingAPI instance to use the specified API key for authorization.
//
// When working with tonapi.io, you should consider getting an API key at https://tonconsole.com/
// because tonapi.io has per-ip limits for sse and websocket connections.
func WithStreamingToken(apiKey string) StreamingOption {
	return func(o *StreamingOptions) {
		o.apiKey = apiKey
	}
}

func WithStreamingLogger(logger Logger) StreamingOption {
	return func(o *StreamingOptions) {
		o.logger = logger
	}
}

func NewStreamingAPI(opts ...StreamingOption) *StreamingAPI {
	options := &StreamingOptions{
		endpoint: TonApiURL,
		logger:   &noopLogger{},
	}
	for _, o := range opts {
		o(options)
	}
	return &StreamingAPI{
		logger:   options.logger,
		apiKey:   options.apiKey,
		endpoint: options.endpoint,
	}
}

// Websocket contains methods to configure a websocket connection to receive particular events from tonapi.io
// happening in the TON blockchain.
type Websocket interface {
	// SubscribeToTransactions subscribes to notifications about new transactions for the specified accounts.
	SubscribeToTransactions(accounts []string) error
	// UnsubscribeFromTransactions unsubscribes from notifications about new transactions for the specified accounts.
	UnsubscribeFromTransactions(accounts []string) error

	// SubscribeToTraces subscribes to notifications about new traces for the specified accounts.
	SubscribeToTraces(accounts []string) error
	// UnsubscribeFromTraces unsubscribes from notifications about new traces for the specified accounts.
	UnsubscribeFromTraces(accounts []string) error
	// SubscribeToMempool subscribes to notifications about new messages in the TON network.

	SubscribeToMempool() error
	// UnsubscribeFromMempool unsubscribes from notifications about new messages in the TON network.
	UnsubscribeFromMempool() error

	// SetMempoolHandler defines a callback that will be called when a new mempool event is received.
	SetMempoolHandler(handler MempoolHandler)
	// SetTransactionHandler defines a callback that will be called when a new transaction event is received.
	SetTransactionHandler(handler TransactionHandler)
	// SetTraceHandler defines a callback that will be called when a new trace event is received.
	SetTraceHandler(handler TraceHandler)
}

// WebsocketConfigurator configures an open websocket connection.
// If it returns an error,
// the connection will be closed and WebsocketHandleRequests will quit returning the error.
type WebsocketConfigurator func(ws Websocket) error

// WebsocketHandleRequests opens a new websocket connection to tonapi.io and run JSON RPC protocol.
// The advantage of using this method over SubscribeTo* methods is that you can subscribe and unsubscribe to/from multiple events
// at any time and in any order.
//
// The given configurator runs in a dedicated goroutine independently of the connection's main loop and can be used in two ways:
//  1. to configure the connection and quit immediately, returning nil.
//     In this case, the connection is configured only once and there is no way to reconfigure it later.
//  2. to run a loop which reconfigures the connection once you need to subscribe/unsubscribe to new events.
//     If the configurator returns an error, the connection will be closed and the function will return the error.
//
// The configurator is called when the underlying websocket connection is established.
func (s *StreamingAPI) WebsocketHandleRequests(ctx context.Context, fn WebsocketConfigurator) error {
	ws, err := websocketConnect(ctx, s.endpoint, s.apiKey)
	if err != nil {
		return err
	}
	return ws.runJsonRPC(ctx, fn)
}

// SubscribeToTraces opens a new sse connection to tonapi.io and subscribes to new traces for the specified accounts.
// When a new trace is received, the handler will be called.
// If accounts is empty, all traces for all accounts will be received.
// This function returns an error when the underlying connection fails or context is canceled.
// No automatic reconnection is performed.
func (s *StreamingAPI) SubscribeToTraces(ctx context.Context, accounts []string, handler TraceHandler) error {
	accountsQueryStr := "ALL"
	if len(accounts) > 0 {
		accountsQueryStr = strings.Join(accounts, ",")
	}
	url := fmt.Sprintf("%s/v2/sse/accounts/traces?accounts=%s", s.endpoint, accountsQueryStr)

	return s.subscribe(ctx, url, s.apiKey, func(data []byte) {
		eventData := TraceEventData{}
		if err := json.Unmarshal(data, &eventData); err != nil {
			// this should never happen but anyway
			s.logger.Errorf("sse connection received invalid trace event data: %v", err)
		}
		handler(eventData)
	})
}

// SubscribeToMempool opens a new sse connection to tonapi.io and subscribes to new mempool events.
// When a new mempool event is received, the handler will be called.
// This function returns an error when the underlying connection fails or context is canceled.
// No automatic reconnection is performed.
func (s *StreamingAPI) SubscribeToMempool(ctx context.Context, handler MempoolHandler) error {
	url := fmt.Sprintf("%s/v2/sse/mempool", s.endpoint)
	return s.subscribe(ctx, url, s.apiKey, func(data []byte) {
		eventData := MempoolEventData{}
		if err := json.Unmarshal(data, &eventData); err != nil {
			// this should never happen but anyway
			s.logger.Errorf("sse connection received invalid mempool event data: %v", err)
		}
		handler(eventData)
	})
}

// SubscribeToTransactions opens a new sse connection to tonapi.io and subscribes to new transactions for the specified accounts.
// When a new transaction is received, the handler will be called.
// If accounts is empty, all traces for all accounts will be received.
// This function returns an error when the underlying connection fails or context is canceled.
// No automatic reconnection is performed.
func (s *StreamingAPI) SubscribeToTransactions(ctx context.Context, accounts []string, handler TransactionHandler) error {
	accountsQueryStr := "ALL"
	if len(accounts) > 0 {
		accountsQueryStr = strings.Join(accounts, ",")
	}
	url := fmt.Sprintf("%s/v2/sse/accounts/transactions?accounts=%s", s.endpoint, accountsQueryStr)
	return s.subscribe(ctx, url, s.apiKey, func(data []byte) {
		eventData := TransactionEventData{}
		if err := json.Unmarshal(data, &eventData); err != nil {
			// this should never happen but anyway
			s.logger.Errorf("sse connection received invalid transaction event data: %v", err)
		}
		handler(eventData)
	})
}

func (s *StreamingAPI) subscribe(ctx context.Context, url string, apiKey string, handler func(data []byte)) error {
	client := sse.NewClient(url)
	if len(apiKey) > 0 {
		client.Headers = map[string]string{
			"Authorization": fmt.Sprintf("bearer %s", s.apiKey),
		}
	}
	return client.SubscribeWithContext(ctx, "", func(msg *sse.Event) {
		switch string(msg.Event) {
		case "heartbeat":
			return
		case "message":
			handler(msg.Data)
		}
	})
}
