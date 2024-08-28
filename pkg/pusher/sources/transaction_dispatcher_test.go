package sources

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/arnac-io/opentonapi/internal/g"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
)

func Test_transactionDispatcher_registerSubscriber(t *testing.T) {
	tests := []struct {
		name            string
		options         []SubscribeToTransactionsOptions
		wantAllAccounts map[subscriberID]struct{}
		wantAccounts    map[tongo.AccountID]map[subscriberID]struct{}
	}{
		{
			name: "all accounts",
			options: []SubscribeToTransactionsOptions{
				{AllAccounts: true},
			},
			wantAllAccounts: map[subscriberID]struct{}{
				1: {},
			},
			wantAccounts: map[tongo.AccountID]map[subscriberID]struct{}{},
		},
		{
			name: "several accounts",
			options: []SubscribeToTransactionsOptions{
				{
					Accounts: []tongo.AccountID{
						ton.MustParseAccountID("-1:3333333333333333333333333333333333333333333333333333333333333333"),
						ton.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555"),
					},
				},
				{
					Accounts: []tongo.AccountID{
						ton.MustParseAccountID("0:7d45ba83250e5ab0307def9c2d4d322515ad16195b9f3b022efa00ebd3dda65d"),
						ton.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555"),
					},
				},
			},
			wantAllAccounts: map[subscriberID]struct{}{},
			wantAccounts: map[tongo.AccountID]map[subscriberID]struct{}{
				ton.MustParseAccountID("-1:3333333333333333333333333333333333333333333333333333333333333333"): {
					1: {},
				},
				ton.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555"): {
					1: {},
					2: {},
				},
				ton.MustParseAccountID("0:7d45ba83250e5ab0307def9c2d4d322515ad16195b9f3b022efa00ebd3dda65d"): {
					2: {},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			disp := NewTransactionDispatcher(logger)
			var cancels []CancelFn
			for _, opts := range tt.options {
				cancelFn := disp.RegisterSubscriber(func(eventData []byte) {}, opts)
				require.NotNil(t, cancelFn)
				cancels = append(cancels, cancelFn)
			}
			allAccounts := map[subscriberID]struct{}{}
			for subID := range disp.allAccounts {
				allAccounts[subID] = struct{}{}
			}
			require.Equal(t, tt.wantAllAccounts, allAccounts)

			accounts := map[tongo.AccountID]map[subscriberID]struct{}{}
			for account, subscribers := range disp.accounts {
				accounts[account] = map[subscriberID]struct{}{}
				for subID := range subscribers {
					accounts[account][subID] = struct{}{}
				}
			}
			require.Equal(t, tt.wantAccounts, accounts)

			for _, cancel := range cancels {
				cancel()
			}
			require.Equal(t, 0, len(disp.allAccounts))
			require.Equal(t, 0, len(disp.options))
			require.Equal(t, 0, len(disp.accounts))
		})
	}
}

func Test_createDeliveryFnBasedOnOptions(t *testing.T) {
	tests := []struct {
		name       string
		options    SubscribeToTransactionsOptions
		msgOpName  *abi.MsgOpName
		msgOpCode  *uint32
		wantCalled bool
	}{
		{
			name: "all operations - should be called",
			options: SubscribeToTransactionsOptions{
				AllOperations: true,
			},
			msgOpName:  nil,
			msgOpCode:  nil,
			wantCalled: true,
		},
		{
			name: "all operations - should be called",
			options: SubscribeToTransactionsOptions{
				AllOperations: true,
			},
			msgOpName:  g.Pointer("JettonBurn"),
			msgOpCode:  g.Pointer(uint32(0x00112233)),
			wantCalled: true,
		},
		{
			name: "op code match - should be called",
			options: SubscribeToTransactionsOptions{
				Operations: []string{
					"0x00112233",
				},
			},
			msgOpName:  g.Pointer("JettonBurn"),
			msgOpCode:  g.Pointer(uint32(0x00112233)),
			wantCalled: true,
		},
		{
			name: "op name match - should be called",
			options: SubscribeToTransactionsOptions{
				Operations: []string{
					"0x00112233",
					"JettonBurn",
				},
			},
			msgOpName:  g.Pointer("JettonBurn"),
			msgOpCode:  g.Pointer(uint32(0x00112244)),
			wantCalled: true,
		},
		{
			name: "no match",
			options: SubscribeToTransactionsOptions{
				Operations: []string{
					"0x00112233",
				},
			},
			msgOpName:  g.Pointer("JettonBurn"),
			msgOpCode:  g.Pointer(uint32(0x00112244)),
			wantCalled: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isCalled := false
			deliveryFn := createTxDeliveryFnBasedOnOptions(func(eventData []byte) {
				isCalled = true
			}, tt.options)

			deliveryFn([]byte{}, tt.msgOpName, tt.msgOpCode)

			require.Equal(t, tt.wantCalled, isCalled)
		})
	}
}
