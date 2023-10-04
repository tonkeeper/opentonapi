package sources

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/cache"
	"github.com/tonkeeper/opentonapi/pkg/core"
)

func TestTracer_putTraceInCache(t1 *testing.T) {
	tests := []struct {
		name        string
		hash        string
		wantSuccess bool
		wantKeys    map[string]struct{}
	}{
		{
			name:        "already in cache",
			hash:        "100",
			wantSuccess: false,
			wantKeys: map[string]struct{}{
				"100": {},
				"101": {},
			},
		},
		{
			name:        "all good",
			hash:        "102",
			wantSuccess: true,
			wantKeys: map[string]struct{}{
				"100": {},
				"101": {},
				"102": {},
			},
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &Tracer{
				logger:     zap.L(),
				traceCache: cache.NewLRUCache[string, struct{}](10000, "tracer_trace_cache"),
			}
			t.putTraceInCache("100")
			t.putTraceInCache("101")
			success := t.putTraceInCache(tt.hash)
			require.Equal(t1, tt.wantSuccess, success)

			keys := make(map[string]struct{})
			for _, key := range t.traceCache.Keys() {
				keys[key] = struct{}{}
			}
			require.Equal(t1, tt.wantKeys, keys)
		})
	}
}

type mockDispatcher struct {
	OnDispatch func(accountIDs []tongo.AccountID, event []byte)
}

func (m *mockDispatcher) dispatch(accountIDs []tongo.AccountID, event []byte) {
	m.OnDispatch(accountIDs, event)
}

func (m *mockDispatcher) registerSubscriber(fn DeliveryFn, options SubscribeToTraceOptions) CancelFn {
	panic("implement me")
}

var _ dispatcher = (*mockDispatcher)(nil)

func TestTracer_dispatch(t1 *testing.T) {
	tests := []struct {
		name           string
		trace          *core.Trace
		dispatchCalled bool
		wantAccounts   map[tongo.AccountID]struct{}
	}{
		{
			name: "trace already processed",
			trace: &core.Trace{
				Transaction: core.Transaction{
					TransactionID: core.TransactionID{
						Hash: tongo.MustParseHash("8936feaad876259c486c578f025cbd02d3017e38f64299222b22c7fd9b21c14b"),
					},
				},
			},
			dispatchCalled: false,
			wantAccounts:   map[tongo.AccountID]struct{}{},
		},
		{
			name: "all good",
			trace: &core.Trace{
				Transaction: core.Transaction{
					TransactionID: core.TransactionID{
						Hash:    tongo.MustParseHash("000000000000259c486c578f025cbd02d3017e38f64299222b22c7fd9b21c100"),
						Account: tongo.MustParseAccountID("0:dd61300e0060f80233363b3b4a0f3b27ad03b19cc4bec6ec798aab0b3e479eba"),
					},
				},
			},
			wantAccounts: map[tongo.AccountID]struct{}{
				tongo.MustParseAccountID("0:dd61300e0060f80233363b3b4a0f3b27ad03b19cc4bec6ec798aab0b3e479eba"): {},
			},
			dispatchCalled: true,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			dispatchCalled := false
			accounts := map[tongo.AccountID]struct{}{}
			disp := &mockDispatcher{
				OnDispatch: func(accountIDs []tongo.AccountID, event []byte) {
					dispatchCalled = true
					for _, account := range accountIDs {
						accounts[account] = struct{}{}
					}
				},
			}
			t := &Tracer{
				logger:     zap.L(),
				dispatcher: disp,
				traceCache: cache.NewLRUCache[string, struct{}](10000, "tracer_trace_cache"),
			}
			t.putTraceInCache("8936feaad876259c486c578f025cbd02d3017e38f64299222b22c7fd9b21c14b")
			t.dispatch(tt.trace)

			require.Equal(t1, tt.dispatchCalled, dispatchCalled)
			require.Equal(t1, tt.wantAccounts, accounts)
		})
	}
}
