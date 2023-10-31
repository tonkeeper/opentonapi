package tonapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

// JsonRPCRequest represents a request in the JSON-RPC protocol supported by "/v2/websocket" endpoint.
type JsonRPCRequest struct {
	ID      uint64   `json:"id,omitempty"`
	JSONRPC string   `json:"jsonrpc,omitempty"`
	Method  string   `json:"method,omitempty"`
	Params  []string `json:"params,omitempty"`
}

// JsonRPCResponse represents a response in the JSON-RPC protocol supported by "/v2/websocket" endpoint.
type JsonRPCResponse struct {
	ID      uint64          `json:"id,omitempty"`
	JSONRPC string          `json:"jsonrpc,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type websocketConnection struct {
	// mu protects the handler fields below.
	mu                 sync.Mutex
	conn               *websocket.Conn
	mempoolHandler     MempoolHandler
	transactionHandler TransactionHandler
	traceHandler       TraceHandler
}

func (w *websocketConnection) SubscribeToTransactions(accounts []string) error {
	request := JsonRPCRequest{
		ID:      1,
		JSONRPC: "2.0",
		Method:  "subscribe_account",
		Params:  accounts,
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteJSON(request)
}

func (w *websocketConnection) SubscribeToTraces(accounts []string) error {
	request := JsonRPCRequest{
		ID:      1,
		JSONRPC: "2.0",
		Method:  "subscribe_trace",
		Params:  accounts,
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteJSON(request)
}

func (w *websocketConnection) SubscribeToMempool() error {
	request := JsonRPCRequest{
		ID:      1,
		JSONRPC: "2.0",
		Method:  "subscribe_mempool",
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteJSON(request)
}

func (w *websocketConnection) SetMempoolHandler(handler MempoolHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.mempoolHandler = handler
}

func (w *websocketConnection) SetTransactionHandler(handler TransactionHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.transactionHandler = handler
}

func (w *websocketConnection) SetTraceHandler(handler TraceHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.traceHandler = handler
}

func websocketConnect(ctx context.Context, endpoint string, apiKey string) (*websocketConnection, error) {
	header := http.Header{}
	if len(apiKey) > 0 {
		header.Set("Authorization", "bearer "+apiKey)
	}
	endpointUrl, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	switch endpointUrl.Scheme {
	case "http":
		endpointUrl.Scheme = "ws"
	case "https":
		endpointUrl.Scheme = "wss"
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, fmt.Sprintf("%s/v2/websocket", endpointUrl.String()), header)
	if err != nil {
		return nil, err
	}
	return &websocketConnection{
		conn:               conn,
		mempoolHandler:     func(data MempoolEventData) {},
		transactionHandler: func(data TransactionEventData) {},
		traceHandler:       func(data TraceEventData) {},
	}, nil
}

func (w *websocketConnection) runJsonRPC(ctx context.Context, fn WebsocketConfigurator) error {
	defer w.conn.Close()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return fn(w)
	})
	g.Go(func() error {
		for {
			_, msg, err := w.conn.ReadMessage()
			if err != nil {
				return err
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var response JsonRPCResponse
			if err := json.Unmarshal(msg, &response); err != nil {
				return err
			}
			switch response.Method {
			case "trace":
				var traceEvent TraceEventData
				if err := json.Unmarshal(response.Params, &traceEvent); err != nil {
					return err
				}
				w.processHandler(func() {
					w.traceHandler(traceEvent)
				})
			case "account_transaction":
				var txEvent TransactionEventData
				if err := json.Unmarshal(response.Params, &txEvent); err != nil {
					return err
				}
				w.processHandler(func() {
					w.transactionHandler(txEvent)
				})
			case "mempool_message":
				var mempoolEvent MempoolEventData
				if err := json.Unmarshal(response.Params, &mempoolEvent); err != nil {
					return err
				}
				w.processHandler(func() {
					w.mempoolHandler(mempoolEvent)
				})
			}
		}
	})
	return g.Wait()
}

func (w *websocketConnection) processHandler(fn func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	fn()
}
