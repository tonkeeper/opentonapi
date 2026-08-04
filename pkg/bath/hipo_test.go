package bath

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func hipoOpBubble(opCode uint32) *Bubble {
	op := opCode
	return &Bubble{Info: BubbleTx{opCode: &op}}
}

// TestHipoUnstakeNotRolledBack guards the negative case of the unstake straws without
// needing a trace: a rolled-back unstake looks exactly like a deferred one except for the
// proxy_rollback_unstake answer, and must not be reported as a withdraw request.
func TestHipoUnstakeNotRolledBack(t *testing.T) {
	tests := []struct {
		name     string
		children []*Bubble
		want     bool
	}{
		{
			name:     "instant unstake pays out",
			children: []*Bubble{hipoOpBubble(hipoProxyTokensBurnedMsgOpCode)},
			want:     true,
		},
		{
			name:     "deferred unstake mints a bill",
			children: []*Bubble{hipoOpBubble(hipoMintBillMsgOpCode)},
			want:     true,
		},
		{
			name:     "rolled back unstake",
			children: []*Bubble{hipoOpBubble(hipoProxyRollbackUnstakeMsgOpCode)},
			want:     false,
		},
		{
			name:     "no answer at all",
			children: nil,
			want:     true,
		},
		{
			name:     "non-transaction children are ignored",
			children: []*Bubble{{Info: BubbleContractDeploy{}}},
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bubble := &Bubble{Info: BubbleTx{}, Children: tt.children}
			require.Equal(t, tt.want, hipoUnstakeNotRolledBack(bubble))
		})
	}
}

// TestHipoBillAssigned covers both shapes a bill transaction can have by the time the Hipo
// straws run: already merged into an NFT transfer by NftTransferNotifyStraw, or still a
// plain assign_bill transaction.
func TestHipoBillAssigned(t *testing.T) {
	require.True(t, hipoBillAssigned(&Bubble{Info: BubbleNftTransfer{}}))
	require.True(t, hipoBillAssigned(hipoOpBubble(hipoAssignBillMsgOpCode)))
	require.False(t, hipoBillAssigned(hipoOpBubble(hipoMintBillMsgOpCode)))
	require.False(t, hipoBillAssigned(&Bubble{Info: BubbleTx{}}))
	require.False(t, hipoBillAssigned(&Bubble{Info: BubbleJettonBurn{}}))
}
