package blockchain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
