package sources

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo"
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
						tongo.MustParseAccountID("-1:3333333333333333333333333333333333333333333333333333333333333333"),
						tongo.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555"),
					},
				},
				{
					Accounts: []tongo.AccountID{
						tongo.MustParseAccountID("0:7d45ba83250e5ab0307def9c2d4d322515ad16195b9f3b022efa00ebd3dda65d"),
						tongo.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555"),
					},
				},
			},
			wantAllAccounts: map[subscriberID]struct{}{},
			wantAccounts: map[tongo.AccountID]map[subscriberID]struct{}{
				tongo.MustParseAccountID("-1:3333333333333333333333333333333333333333333333333333333333333333"): {
					1: {},
				},
				tongo.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555"): {
					1: {},
					2: {},
				},
				tongo.MustParseAccountID("0:7d45ba83250e5ab0307def9c2d4d322515ad16195b9f3b022efa00ebd3dda65d"): {
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
