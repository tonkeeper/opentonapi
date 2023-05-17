package bath

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo"

	"github.com/tonkeeper/opentonapi/pkg/core"
)

func TestBubbleTx_ToAction(t *testing.T) {
	tests := []struct {
		name string
		tx   BubbleTx
		want *Action
	}{
		{
			tx: BubbleTx{
				external:        true,
				success:         true,
				transactionType: core.TickTockTx,
				inputAmount:     0,
				account: Account{
					Address: tongo.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555"),
				},
			},
			want: &Action{
				SmartContractExec: &SmartContractAction{
					Executor:  tongo.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555"),
					Contract:  tongo.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555"),
					Operation: "Tick-tock",
				},
				Success: true,
				Type:    SmartContractExec,
				SimplePreview: SimplePreview{
					Accounts: []tongo.AccountID{
						tongo.MustParseAccountID("-1:5555555555555555555555555555555555555555555555555555555555555555"),
					},
					MessageID: "smartContractExecAction",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := tt.tx.ToAction(nil)
			require.Equal(t, tt.want, action)
		})
	}
}
