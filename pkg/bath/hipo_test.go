package bath

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

func hipoTx(account tongo.AccountID, operation string, children ...*Bubble) *Bubble {
	return &Bubble{
		Info: BubbleTx{
			success:     true,
			account:     Account{Address: account},
			decodedBody: &core.DecodedMessageBody{Operation: operation},
		},
		Children:  children,
		ValueFlow: newValueFlow(),
	}
}

// hipoUnstake builds an hGRAM burn whose reserve_tokens transaction on the treasury answers
// with the given children.
func hipoUnstake(master tongo.AccountID, treasuryAnswer ...*Bubble) *Bubble {
	return &Bubble{
		Info:      BubbleJettonBurn{master: master},
		ValueFlow: newValueFlow(),
		Children: []*Bubble{
			hipoTx(references.HipoParent, abi.HipoFinanceProxyReserveTokensMsgOp,
				hipoTx(references.HipoTreasury, abi.HipoFinanceReserveTokensMsgOp, treasuryAnswer...),
			),
		},
	}
}

// The rollback case has no golden trace: the last one on mainnet is older than public
// liteservers keep, so the straw is exercised on a synthetic bubble tree instead.
func TestWithdrawHipoStakeRequestStraw(t *testing.T) {
	var someone tongo.AccountID
	tests := []struct {
		name   string
		bubble *Bubble
		want   bool
	}{
		{
			name: "deferred unstake mints a bill",
			bubble: hipoUnstake(references.HipoParent,
				hipoTx(someone, abi.HipoFinanceMintBillMsgOp,
					hipoTx(someone, abi.HipoFinanceAssignBillMsgOp),
				),
			),
			want: true,
		},
		{
			name:   "instant unstake pays out",
			bubble: hipoUnstake(references.HipoParent, hipoTx(references.HipoParent, abi.HipoFinanceProxyTokensBurnedMsgOp)),
			want:   true,
		},
		{
			name:   "rolled back unstake",
			bubble: hipoUnstake(references.HipoParent, hipoTx(references.HipoParent, abi.HipoFinanceProxyRollbackUnstakeMsgOp)),
			want:   false,
		},
		{
			name:   "burn of another jetton",
			bubble: hipoUnstake(someone, hipoTx(references.HipoParent, abi.HipoFinanceProxyTokensBurnedMsgOp)),
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, WithdrawHipoStakeRequestStraw.Merge(tt.bubble))
		})
	}
}
