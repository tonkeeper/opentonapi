package websocket

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo"

	"github.com/tonkeeper/opentonapi/pkg/pusher/sources"
)

func Test_session_subscribeToTransactions(t *testing.T) {
	tests := []struct {
		name              string
		subscriptionLimit int
		params            []string
		want              string
		wantSubscriptions map[tongo.AccountID]struct{}
		wantEvents        int
	}{
		{
			name:              "all good",
			subscriptionLimit: 3,
			params: []string{
				"-1:5555555555555555555555555555555555555555555555555555555555555555",
				"0:5555555555555555555555555555555555555555555555555555555555555555",
			},
			wantSubscriptions: map[tongo.AccountID]struct{}{
				tongo.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555"): {},
				tongo.MustParseAccountID("0:5555555555555555555555555555555555555555555555555555555555555555"):  {},
			},
			want:       `success! 2 new subscriptions created`,
			wantEvents: 2,
		},
		{
			name:              "subscribe to account multiple times",
			subscriptionLimit: 3,
			params: []string{
				"-1:5555555555555555555555555555555555555555555555555555555555555555",
				"-1:5555555555555555555555555555555555555555555555555555555555555555",
				"-1:5555555555555555555555555555555555555555555555555555555555555555",
			},
			wantSubscriptions: map[tongo.AccountID]struct{}{
				tongo.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555"): {},
			},
			want:       `success! 1 new subscriptions created`,
			wantEvents: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &session{
				eventCh:           make(chan event, 10),
				txSubscriptions:   map[tongo.AccountID]sources.CancelFn{},
				subscriptionLimit: tt.subscriptionLimit,
				txSource: &mockTxSource{
					OnSubscribeToTransactions: func(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToTransactionsOptions) sources.CancelFn {
						deliveryFn([]byte("msg"))
						return func() {}
					},
				},
			}
			msg := s.subscribeToTransactions(context.Background(), tt.params)
			require.Equal(t, tt.want, msg)
			subs := make(map[tongo.AccountID]struct{})
			for sub := range s.txSubscriptions {
				subs[sub] = struct{}{}
			}
			require.Equal(t, tt.wantSubscriptions, subs)
			close(s.eventCh)
			require.Equal(t, tt.wantEvents, len(s.eventCh))

		})
	}
}
