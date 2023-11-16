package websocket

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/pusher/sources"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
)

func Test_session_subscribeToTransactions(t *testing.T) {
	tests := []struct {
		name              string
		subscriptionLimit int
		params            []string
		want              string
		wantSubscriptions map[tongo.AccountID]struct{}
		wantEvents        int
		wantOptions       []sources.SubscribeToTransactionsOptions
	}{
		{
			name:              "all good",
			subscriptionLimit: 3,
			params: []string{
				"-1:5555555555555555555555555555555555555555555555555555555555555555",
				"0:5555555555555555555555555555555555555555555555555555555555555555;operations=JettonBurn,0x00112233,JettonMint",
			},
			wantSubscriptions: map[tongo.AccountID]struct{}{
				tongo.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555"): {},
				tongo.MustParseAccountID("0:5555555555555555555555555555555555555555555555555555555555555555"):  {},
			},
			want:       `success! 2 new subscriptions created`,
			wantEvents: 2,
			wantOptions: []sources.SubscribeToTransactionsOptions{
				{
					Accounts:      []tongo.AccountID{ton.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555")},
					AllOperations: true,
				},
				{
					Accounts:   []tongo.AccountID{ton.MustParseAccountID("0:5555555555555555555555555555555555555555555555555555555555555555")},
					Operations: []string{"JettonBurn", "0x00112233", "JettonMint"},
				},
			},
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
			wantOptions: []sources.SubscribeToTransactionsOptions{
				{
					Accounts:      []tongo.AccountID{ton.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555")},
					AllOperations: true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var options []sources.SubscribeToTransactionsOptions
			s := &session{
				eventCh:           make(chan event, 10),
				txSubscriptions:   map[tongo.AccountID]sources.CancelFn{},
				subscriptionLimit: tt.subscriptionLimit,
				txSource: &mockTxSource{
					OnSubscribeToTransactions: func(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToTransactionsOptions) sources.CancelFn {
						options = append(options, opts)
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
			sort.Slice(options, func(i, j int) bool {
				return options[i].Accounts[0].String() < options[j].Accounts[0].String()
			})
			require.Equal(t, tt.wantOptions, options)
		})
	}
}

func Test_session_sendEvent(t *testing.T) {
	tests := []struct {
		name           string
		sendEventCount int
		wantEvents     map[string]struct{}
	}{
		{
			name:           "some events dropped",
			sendEventCount: 10,
			wantEvents: map[string]struct{}{
				"0": {},
				"1": {},
				"2": {},
				"3": {},
			},
		},
		{
			name:           "all events delivered",
			sendEventCount: 3,
			wantEvents: map[string]struct{}{
				"0": {},
				"1": {},
				"2": {},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &session{
				logger:  zap.L(),
				eventCh: make(chan event, 4),
			}
			for i := 0; i < tt.sendEventCount; i++ {
				s.sendEvent(event{Method: fmt.Sprintf("%v", i)})
			}

			close(s.eventCh)
			events := make(map[string]struct{})
			for event := range s.eventCh {
				events[event.Method] = struct{}{}
			}
			require.Equal(t, tt.wantEvents, events)
		})
	}
}

func Test_processParam(t *testing.T) {
	tests := []struct {
		name    string
		param   string
		want    *accountOptions
		wantErr bool
	}{
		{
			name:  "param contains an account",
			param: "-1:5555555555555555555555555555555555555555555555555555555555555555",
			want: &accountOptions{
				Account: ton.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555"),
			},
		},
		{
			name:  "param contains an account with operations",
			param: "-1:5555555555555555555555555555555555555555555555555555555555555555;operations=JettonBurn,0x00112233,JettonMint",
			want: &accountOptions{
				Account:    ton.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555"),
				Operations: []string{"JettonBurn", "0x00112233", "JettonMint"},
			},
		},
		{
			name:  "param contains an account with empty operations",
			param: "-1:5555555555555555555555555555555555555555555555555555555555555555;operations=",
			want: &accountOptions{
				Account: ton.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555"),
			},
		},
		{
			name:    "param contains an account with malformed operations",
			param:   "-1:5555555555555555555555555555555555555555555555555555555555555555;ops=JettonBurn,0x00112233,JettonMint",
			wantErr: true,
		},
		{
			name:    "param contains a malformed account",
			param:   "-15555555555555555555555555555555555555555555555555555555555555555",
			wantErr: true,
		},
		{
			name:    "param contains an account with malformed operations",
			param:   "-1:5555555555555555555555555555555555555555555555555555555555555555;JettonBurn,0x00112233,JettonMint",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options, err := processParam(tt.param)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, options)
		})
	}
}
