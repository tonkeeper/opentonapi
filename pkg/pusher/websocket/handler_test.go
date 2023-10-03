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
	OnSubscribeToMessages func(ctx context.Context, deliveryFn sources.DeliveryFn) (sources.CancelFn, error)
}

func (m *mockMemPool) SubscribeToMessages(ctx context.Context, deliveryFn sources.DeliveryFn) (sources.CancelFn, error) {
	return m.OnSubscribeToMessages(ctx, deliveryFn)
}

type mockTraceSource struct {
	OnSubscribeToTraces func(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToTraceOptions) sources.CancelFn
}

func (m *mockTraceSource) SubscribeToTraces(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToTraceOptions) sources.CancelFn {
	return m.OnSubscribeToTraces(ctx, deliveryFn, opts)
}

var _ sources.TransactionSource = &mockTxSource{}
var _ sources.MemPoolSource = &mockMemPool{}
var _ sources.TraceSource = &mockTraceSource{}

func TestHandler(t *testing.T) {
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
		OnSubscribeToMessages: func(ctx context.Context, deliveryFn sources.DeliveryFn) (sources.CancelFn, error) {
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
		handler := Handler(logger, source, traceSource, mempool)
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
