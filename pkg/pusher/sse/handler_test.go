package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/pusher/sources"
	"github.com/tonkeeper/tongo"
)

type mockTxSource struct {
	options sources.SubscribeToTransactionsOptions
}

func (m *mockTxSource) SubscribeToTransactions(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToTransactionsOptions) sources.CancelFn {
	m.options = opts
	return nil
}

var _ sources.TransactionSource = (*mockTxSource)(nil)

func TestHandler_SubscribeToTransactions(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantErr     bool
		wantOptions sources.SubscribeToTransactionsOptions
	}{
		{
			name: "all accounts",
			url:  "/transactions?accounts=all",
			wantOptions: sources.SubscribeToTransactionsOptions{
				AllAccounts:   true,
				AllOperations: true,
			},
		},
		{
			name: "specific accounts and operations",
			url:  "/transactions?accounts=0:779dcc815138d9500e449c5291e7f12738c23d575b5310000f6a253bd607384e&operations=JettonBurn,0x11223344,0x55667788,JettonMint",
			wantOptions: sources.SubscribeToTransactionsOptions{
				Accounts: []tongo.AccountID{
					tongo.MustParseAddress("0:779dcc815138d9500e449c5291e7f12738c23d575b5310000f6a253bd607384e").ID,
				},
				Operations: []string{
					"JettonBurn",
					"0x11223344",
					"0x55667788",
					"JettonMint",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &mockTxSource{}
			h := &Handler{
				txSource: source,
			}
			request := httptest.NewRequest(http.MethodGet, tt.url, nil)
			err := h.SubscribeToTransactions(&session{}, request)
			require.Nil(t, err)
			require.Equal(t, tt.wantOptions, source.options)
		})
	}
}
