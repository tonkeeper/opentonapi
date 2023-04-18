package websocket

import (
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
	OnSubscribeToTransactions func(deliveryFn sources.DeliveryTxFn, opts sources.SubscribeToTransactionsOptions) sources.CancelFn
}

func (m *mockTxSource) SubscribeToTransactions(deliveryFn sources.DeliveryTxFn, opts sources.SubscribeToTransactionsOptions) sources.CancelFn {
	return m.OnSubscribeToTransactions(deliveryFn, opts)
}

var _ sources.TransactionSource = &mockTxSource{}

func TestHandler(t *testing.T) {
	var subscribed atomic.Bool   // to make "go test -race" happy
	var unsubscribed atomic.Bool // to make "go test -race" happy
	source := &mockTxSource{
		OnSubscribeToTransactions: func(deliveryFn sources.DeliveryTxFn, opts sources.SubscribeToTransactionsOptions) sources.CancelFn {
			subscribed.Store(true)
			return func() {
				unsubscribed.Store(true)
			}
		},
	}
	logger, _ := zap.NewDevelopment()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		handler := Handler(logger, source)
		err := handler(writer, request, 0)
		require.Nil(t, err)
	}))
	defer server.Close()

	url := strings.Replace(server.URL, "http", "ws", -1)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.Nil(t, err)

	request := JsonRPCRequest{
		ID:      1,
		JSONRPC: "2.0",
		Method:  "subscribe_account",
		Params: []string{
			"-1:5555555555555555555555555555555555555555555555555555555555555555",
			"0:5555555555555555555555555555555555555555555555555555555555555555",
		},
	}
	err = conn.WriteJSON(request)
	require.Nil(t, err)

	time.Sleep(1 * time.Second)

	msgType, msg, err := conn.ReadMessage()
	require.Nil(t, err)
	require.Equal(t, websocket.TextMessage, msgType)
	expectedResponse := []byte(`{"id":1,"jsonrpc":"2.0","method":"subscribe_account","result":"success! 2 new subscriptions created"}` + "\n")
	require.Equal(t, expectedResponse, msg)
	require.True(t, subscribed.Load())
	require.False(t, unsubscribed.Load())
	conn.Close()
	time.Sleep(1 * time.Second)
	require.True(t, unsubscribed.Load())

}
