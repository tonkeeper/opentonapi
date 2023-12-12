package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/pusher/sources"
)

type mockTxSource struct {
	OnSubscribeToTransactions func(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToTransactionsOptions) sources.CancelFn
}

func (m *mockTxSource) SubscribeToTransactions(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToTransactionsOptions) sources.CancelFn {
	return m.OnSubscribeToTransactions(ctx, deliveryFn, opts)
}

type mockMemPool struct {
	OnSubscribeToMessages func(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToMempoolOptions) (sources.CancelFn, error)
}

func (m *mockMemPool) SubscribeToMessages(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToMempoolOptions) (sources.CancelFn, error) {
	return m.OnSubscribeToMessages(ctx, deliveryFn, opts)
}

type mockTraceSource struct {
	OnSubscribeToTraces func(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToTraceOptions) sources.CancelFn
}

func (m *mockTraceSource) SubscribeToTraces(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToTraceOptions) sources.CancelFn {
	return m.OnSubscribeToTraces(ctx, deliveryFn, opts)
}

type mockBlockSource struct {
	OnSubscribeToBlocks func(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToBlocksOptions) sources.CancelFn
}

func (m *mockBlockSource) SubscribeToBlocks(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToBlocksOptions) sources.CancelFn {
	return m.OnSubscribeToBlocks(ctx, deliveryFn, opts)
}

var _ sources.TransactionSource = &mockTxSource{}
var _ sources.MemPoolSource = &mockMemPool{}
var _ sources.TraceSource = &mockTraceSource{}
var _ sources.BlockSource = &mockBlockSource{}

func TestHandler_UnsubscribeWhenConnectionIsClosed(t *testing.T) {
	var txSubscribed atomic.Bool   // to make "go test -race" happy
	var txUnsubscribed atomic.Bool // to make "go test -race" happy
	source := &mockTxSource{
		OnSubscribeToTransactions: func(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToTransactionsOptions) sources.CancelFn {
			txSubscribed.Store(true)
			return func() {
				txUnsubscribed.Store(true)
			}
		},
	}
	var memPoolSubscribed atomic.Bool   // to make "go test -race" happy
	var memPoolUnsubscribed atomic.Bool // to make "go test -race" happy
	mempool := &mockMemPool{
		OnSubscribeToMessages: func(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToMempoolOptions) (sources.CancelFn, error) {
			memPoolSubscribed.Store(true)
			return func() {
				memPoolUnsubscribed.Store(true)
			}, nil
		},
	}
	var traceSubscribed atomic.Bool   // to make "go test -race" happy
	var traceUnsubscribed atomic.Bool // to make "go test -race" happy
	traceSource := &mockTraceSource{
		OnSubscribeToTraces: func(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToTraceOptions) sources.CancelFn {
			traceSubscribed.Store(true)
			return func() {
				traceUnsubscribed.Store(true)
			}
		},
	}
	logger, _ := zap.NewDevelopment()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		handler := Handler(logger, source, traceSource, mempool, nil)
		err := handler(writer, request, 0, false)
		require.Nil(t, err)
	}))
	defer server.Close()

	url := strings.Replace(server.URL, "http", "ws", -1)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.Nil(t, err)

	requests := []JsonRPCRequest{
		{
			ID:      1,
			JSONRPC: "2.0",
			Method:  "subscribe_account",
			Params: []string{
				"-1:5555555555555555555555555555555555555555555555555555555555555555",
				"0:5555555555555555555555555555555555555555555555555555555555555555",
			},
		},
		{
			ID:      2,
			JSONRPC: "2.0",
			Method:  "subscribe_mempool",
		},
		{
			ID:      3,
			JSONRPC: "2.0",
			Method:  "subscribe_trace",
			Params: []string{
				"0:5555555555555555555555555555555555555555555555555555555555555555",
			},
		},
	}
	expectedResponses := [][]byte{
		[]byte(`{"id":1,"jsonrpc":"2.0","method":"subscribe_account","result":"success! 2 new subscriptions created"}` + "\n"),
		[]byte(`{"id":2,"jsonrpc":"2.0","method":"subscribe_mempool","result":"success! you have subscribed to mempool"}` + "\n"),
		[]byte(`{"id":3,"jsonrpc":"2.0","method":"subscribe_trace","result":"success! 1 new subscriptions created"}` + "\n"),
	}

	for i, request := range requests {
		expectedResponse := expectedResponses[i]
		err = conn.WriteJSON(request)
		require.Nil(t, err)

		time.Sleep(1 * time.Second)

		msgType, msg, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, websocket.TextMessage, msgType)

		require.Equal(t, expectedResponse, msg)
	}
	require.True(t, txSubscribed.Load())
	require.False(t, txUnsubscribed.Load())

	require.True(t, memPoolSubscribed.Load())
	require.False(t, memPoolUnsubscribed.Load())

	require.True(t, traceSubscribed.Load())
	require.False(t, traceUnsubscribed.Load())

	conn.Close()
	time.Sleep(1 * time.Second)
	require.True(t, txUnsubscribed.Load())
	require.True(t, memPoolUnsubscribed.Load())
	require.True(t, traceUnsubscribed.Load())
}

func TestHandler_UnsubscribeMethods(t *testing.T) {
	var txSubscribed atomic.Bool   // to make "go test -race" happy
	var txUnsubscribed atomic.Bool // to make "go test -race" happy
	source := &mockTxSource{
		OnSubscribeToTransactions: func(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToTransactionsOptions) sources.CancelFn {
			txSubscribed.Store(true)
			return func() {
				txUnsubscribed.Store(true)
			}
		},
	}
	var memPoolSubscribed atomic.Bool   // to make "go test -race" happy
	var memPoolUnsubscribed atomic.Bool // to make "go test -race" happy
	mempool := &mockMemPool{
		OnSubscribeToMessages: func(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToMempoolOptions) (sources.CancelFn, error) {
			memPoolSubscribed.Store(true)
			return func() {
				memPoolUnsubscribed.Store(true)
			}, nil
		},
	}
	var traceSubscribed atomic.Bool   // to make "go test -race" happy
	var traceUnsubscribed atomic.Bool // to make "go test -race" happy
	traceSource := &mockTraceSource{
		OnSubscribeToTraces: func(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToTraceOptions) sources.CancelFn {
			traceSubscribed.Store(true)
			return func() {
				traceUnsubscribed.Store(true)
			}
		},
	}
	var blockSubscribed atomic.Bool   // to make "go test -race" happy
	var blockUnsubscribed atomic.Bool // to make "go test -race" happy
	blockSource := &mockBlockSource{
		OnSubscribeToBlocks: func(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToBlocksOptions) sources.CancelFn {
			blockSubscribed.Store(true)
			return func() {
				blockUnsubscribed.Store(true)
			}
		},
	}
	logger, _ := zap.NewDevelopment()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		handler := Handler(logger, source, traceSource, mempool, blockSource)
		err := handler(writer, request, 0, false)
		require.Nil(t, err)
	}))
	defer server.Close()

	url := strings.Replace(server.URL, "http", "ws", -1)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.Nil(t, err)

	requests := []JsonRPCRequest{
		{
			ID:      1,
			JSONRPC: "2.0",
			Method:  "subscribe_account",
			Params: []string{
				"-1:5555555555555555555555555555555555555555555555555555555555555555",
				"0:5555555555555555555555555555555555555555555555555555555555555555",
			},
		},
		{
			ID:      2,
			JSONRPC: "2.0",
			Method:  "subscribe_mempool",
		},
		{
			ID:      3,
			JSONRPC: "2.0",
			Method:  "subscribe_trace",
			Params: []string{
				"0:5555555555555555555555555555555555555555555555555555555555555555",
			},
		},
		{
			ID:      4,
			JSONRPC: "2.0",
			Method:  "subscribe_block",
			Params: []string{
				"workchain=-1",
			},
		},
	}
	expectedResponses := [][]byte{
		[]byte(`{"id":1,"jsonrpc":"2.0","method":"subscribe_account","result":"success! 2 new subscriptions created"}` + "\n"),
		[]byte(`{"id":2,"jsonrpc":"2.0","method":"subscribe_mempool","result":"success! you have subscribed to mempool"}` + "\n"),
		[]byte(`{"id":3,"jsonrpc":"2.0","method":"subscribe_trace","result":"success! 1 new subscriptions created"}` + "\n"),
		[]byte(`{"id":4,"jsonrpc":"2.0","method":"subscribe_block","result":"success! you have subscribed to blocks"}` + "\n"),
	}

	for i, request := range requests {
		expectedResponse := expectedResponses[i]
		err = conn.WriteJSON(request)
		require.Nil(t, err)

		time.Sleep(1 * time.Second)

		msgType, msg, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, websocket.TextMessage, msgType)

		require.Equal(t, expectedResponse, msg)
	}
	require.True(t, txSubscribed.Load())
	require.False(t, txUnsubscribed.Load())

	require.True(t, memPoolSubscribed.Load())
	require.False(t, memPoolUnsubscribed.Load())

	require.True(t, traceSubscribed.Load())
	require.False(t, traceUnsubscribed.Load())

	require.True(t, blockSubscribed.Load())
	require.False(t, blockUnsubscribed.Load())

	time.Sleep(1 * time.Second)

	requests = []JsonRPCRequest{
		{
			ID:      10,
			JSONRPC: "2.0",
			Method:  "unsubscribe_account",
			Params: []string{
				"-1:5555555555555555555555555555555555555555555555555555555555555555",
				"-1:3333333333333333333333333333333333333333333333333333333333333333",
			},
		},
		{
			ID:      11,
			JSONRPC: "2.0",
			Method:  "unsubscribe_mempool",
		},
		{
			ID:      12,
			JSONRPC: "2.0",
			Method:  "unsubscribe_trace",
			Params: []string{
				"0:5555555555555555555555555555555555555555555555555555555555555555",
				"0:3333333333333333333333333333333333333333333333333333333333333333",
			},
		},
		{
			ID:      13,
			JSONRPC: "2.0",
			Method:  "unsubscribe_block",
			Params:  []string{},
		},
	}
	expectedResponses = [][]byte{
		[]byte(`{"id":10,"jsonrpc":"2.0","method":"unsubscribe_account","result":"success! 1 subscription(s) removed"}` + "\n"),
		[]byte(`{"id":11,"jsonrpc":"2.0","method":"unsubscribe_mempool","result":"success! you have unsubscribed from mempool"}` + "\n"),
		[]byte(`{"id":12,"jsonrpc":"2.0","method":"unsubscribe_trace","result":"success! 1 subscription(s) removed"}` + "\n"),
		[]byte(`{"id":13,"jsonrpc":"2.0","method":"unsubscribe_block","result":"success! you have unsubscribed from blocks"}` + "\n"),
	}

	for i, request := range requests {
		expectedResponse := expectedResponses[i]
		err = conn.WriteJSON(request)
		require.Nil(t, err)

		time.Sleep(1 * time.Second)

		msgType, msg, err := conn.ReadMessage()
		require.Nil(t, err)
		require.Equal(t, websocket.TextMessage, msgType)

		require.Equal(t, expectedResponse, msg)
	}

	require.True(t, txUnsubscribed.Load())
	require.True(t, memPoolUnsubscribed.Load())
	require.True(t, traceUnsubscribed.Load())
	require.True(t, blockUnsubscribed.Load())
}
