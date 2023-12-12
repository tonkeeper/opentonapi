package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/pusher/sources"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
)

type mockTxSource struct {
	options sources.SubscribeToTransactionsOptions
}

func (m *mockTxSource) SubscribeToTransactions(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToTransactionsOptions) sources.CancelFn {
	m.options = opts
	return nil
}

type mockMemPoolSource struct {
	options sources.SubscribeToMempoolOptions
}

func (m *mockMemPoolSource) SubscribeToMessages(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToMempoolOptions) (sources.CancelFn, error) {
	m.options = opts
	return nil, nil
}

type mockBlockSource struct {
	options sources.SubscribeToBlocksOptions
}

func (m *mockBlockSource) SubscribeToBlocks(ctx context.Context, deliveryFn sources.DeliveryFn, opts sources.SubscribeToBlocksOptions) sources.CancelFn {
	m.options = opts
	return nil
}

var _ sources.TransactionSource = (*mockTxSource)(nil)
var _ sources.MemPoolSource = (*mockMemPoolSource)(nil)
var _ sources.BlockSource = (*mockBlockSource)(nil)

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

func TestHandler_SubscribeToMessages(t *testing.T) {
	var testAccount1 = ton.MustParseAccountID("0:0a95e1d4ebe7860d051f8b861730dbdee1440fd11180211914e0089146580351")
	var testAccount2 = ton.MustParseAccountID("0:0a95e1d4ebe7860d051f8b861730dbdee1440fd11180211914e0089146580352")
	tests := []struct {
		name        string
		url         string
		wantErr     string
		wantOptions sources.SubscribeToMempoolOptions
	}{
		{
			name:        "no accounts",
			url:         "/mempool",
			wantOptions: sources.SubscribeToMempoolOptions{Accounts: nil},
		},
		{
			name: "emulation is on",
			url:  "/mempool?accounts=0:0a95e1d4ebe7860d051f8b861730dbdee1440fd11180211914e0089146580351,0:0a95e1d4ebe7860d051f8b861730dbdee1440fd11180211914e0089146580352",
			wantOptions: sources.SubscribeToMempoolOptions{
				Accounts: []tongo.AccountID{testAccount1, testAccount2},
			},
		},
		{
			name:    "bad accounts parameter",
			url:     "/mempool?accounts=xxx",
			wantErr: `can't decode address xxx`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memPool := &mockMemPoolSource{}
			h := &Handler{
				memPool: memPool,
			}
			request := httptest.NewRequest(http.MethodGet, tt.url, nil)
			err := h.SubscribeToMessages(&session{}, request)
			if tt.wantErr != "" {
				require.NotNil(t, err)
				require.Equal(t, err.Error(), tt.wantErr)
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.wantOptions, memPool.options)
		})
	}
}

func TestHandler_SubscribeToBlocks(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantErr     string
		wantOptions sources.SubscribeToBlocksOptions
	}{
		{
			name: "subscribe to 0 workchain",
			url:  "/blocks?workchain=0",
			wantOptions: sources.SubscribeToBlocksOptions{
				Workchain: g.Pointer(0),
			},
		},
		{
			name: "subscribe to -1 workchain",
			url:  "/blocks?workchain=-1",
			wantOptions: sources.SubscribeToBlocksOptions{
				Workchain: g.Pointer(-1),
			},
		},
		{
			name:    "bad workchain parameter",
			url:     "/blocks?workchain=xxx",
			wantErr: `failed to parse 'workchain' parameter in query`,
		},
		{
			name:        "subscribe to all workchains",
			url:         "/blocks",
			wantOptions: sources.SubscribeToBlocksOptions{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &mockBlockSource{}
			h := &Handler{
				blockSource: source,
			}
			request := httptest.NewRequest(http.MethodGet, tt.url, nil)
			err := h.SubscribeToBlocks(&session{}, request)
			if tt.wantErr != "" {
				require.NotNil(t, err)
				require.Equal(t, err.Error(), tt.wantErr)
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.wantOptions, source.options)
		})
	}
}
