package blockchain

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo/config"
	"github.com/tonkeeper/tongo/liteapi"
)

func TestMsgSender_dropExpiredBatches(t *testing.T) {
	tests := []struct {
		name     string
		batches  []batchOfMessages
		wantBocs []string
	}{
		{
			name: "expire some batches",
			batches: []batchOfMessages{
				{
					Copies: []ExtInMsgCopy{
						{MsgBoc: "1"},
						{MsgBoc: "2"},
					},
					RecvAt: time.Now().Unix(),
				},
				{
					Copies: []ExtInMsgCopy{
						{MsgBoc: "3"},
						{MsgBoc: "4"},
					},
					RecvAt: time.Now().Add(-6 * time.Minute).Unix(),
				},
				{
					Copies: []ExtInMsgCopy{
						{MsgBoc: "5"},
					},
					RecvAt: time.Now().Add(-4 * time.Minute).Unix(),
				},
			},
			wantBocs: []string{"1", "2", "5"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &MsgSender{
				batches: tt.batches,
			}
			ms.dropExpiredBatches()

			var gotBocs []string
			for _, b := range ms.batchesReadyForSending() {
				for _, c := range b.Copies {
					gotBocs = append(gotBocs, c.MsgBoc)
				}
			}
			require.Equal(t, tt.wantBocs, gotBocs)
		})
	}
}

// TestMsgSender_send_AllAttemptsFail is a regression test: send() must
// return the underlying error when every liteserver attempt fails, not
// swallow it. It previously returned nil because `_, err := c.SendMessage(...)`
// inside the retry loop shadowed the function-scoped err instead of assigning it.
func TestMsgSender_send_AllAttemptsFail(t *testing.T) {
	badServer := config.LiteServer{
		Host: "127.0.0.1:1", // nothing listens here; connection must fail
		Key:  "wQEUBFTZKUpFvUVWkT8vTsxRUKWZFqXOMpXFBS2ykRE=",
	}
	client, err := liteapi.NewClient(
		liteapi.WithAsyncConnectionsInit(),
		liteapi.WithLiteServers([]config.LiteServer{badServer}),
	)
	require.NoError(t, err)

	ms := &MsgSender{sendingClients: []*liteapi.Client{client}}
	sendCtx, sendCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sendCancel()

	err = ms.send(sendCtx, []byte("payload content is irrelevant to send()"))
	require.Error(t, err, "send() must surface the failure, not swallow it")
}
