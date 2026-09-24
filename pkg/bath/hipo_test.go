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

// hipoUnstake builds the hGRAM burn of an unstake, whose reserve_tokens transaction on the
// treasury answers with the given children. The straw starts at proxy_reserve_tokens, so
// the burn above it stays a JettonBurn action and is not part of the match.
// The straw pins proxy_reserve_tokens to the jetton master, so the helper builds it there.
func hipoUnstake(treasuryAnswer ...*Bubble) *Bubble {
	return &Bubble{
		Info:      BubbleJettonBurn{master: references.HipoParent},
		ValueFlow: newValueFlow(),
		Children: []*Bubble{
			hipoTx(references.HipoParent, abi.HipoFinanceProxyReserveTokensMsgOp,
				hipoTx(references.HipoTreasury, abi.HipoFinanceReserveTokensMsgOp, treasuryAnswer...),
			),
		},
	}
}

// hipoSettlement builds the treasury transaction that settles one unstake bill of a
// finished round, with whatever the treasury answered it with.
func hipoSettlement(answer ...*Bubble) *Bubble {
	return hipoTx(references.HipoTreasury, abi.HipoFinanceBurnTokensMsgOp, answer...)
}

// None of these have a golden trace: a rollback last happened on mainnet before public
// liteservers kept the block, and a postponed bill needs the treasury to run out of liquid
// GRAM at the exact moment a round ends. They are exercised on synthetic trees instead.
func TestWithdrawHipoStakeRequestStraw(t *testing.T) {
	var someone tongo.AccountID
	tests := []struct {
		name string
		// the proxy_reserve_tokens bubble, i.e. the burn's only child
		bubble  *Bubble
		want    bool
		success bool
	}{
		{
			name: "deferred unstake mints a bill",
			bubble: hipoUnstake(
				hipoTx(someone, abi.HipoFinanceMintBillMsgOp,
					hipoTx(someone, abi.HipoFinanceAssignBillMsgOp),
				),
			).Children[0],
			want:    true,
			success: true,
		},
		{
			name:    "instant unstake pays out",
			bubble:  hipoUnstake(hipoTx(references.HipoParent, abi.HipoFinanceProxyTokensBurnedMsgOp)).Children[0],
			want:    true,
			success: true,
		},
		{
			// The treasury sends the hGRAM straight back, so the request is reported as
			// failed rather than left to look like a completed burn.
			name: "rolled back unstake",
			bubble: hipoUnstake(hipoFrom(hipoTx(references.HipoParent, abi.HipoFinanceProxyRollbackUnstakeMsgOp,
				hipoTx(someone, abi.HipoFinanceRollbackUnstakeMsgOp),
			), references.HipoTreasury)).Children[0],
			want:    true,
			success: false,
		},
		{
			// The comment flow reaches the same reserve_tokens handler through
			// send_unstake_all, with the wallet burning to itself.
			name:    "unstake-all burns to the wallet itself",
			bubble:  hipoUnstake(hipoTx(references.HipoParent, abi.HipoFinanceProxyTokensBurnedMsgOp)).Children[0],
			want:    true,
			success: true,
		},
		{
			name:   "unrelated proxy message",
			bubble: hipoTx(references.HipoParent, abi.HipoFinanceProxySaveCoinsMsgOp),
			want:   false,
		},
		{
			name: "reserve_tokens somewhere other than the treasury",
			bubble: hipoTx(references.HipoParent, abi.HipoFinanceProxyReserveTokensMsgOp,
				hipoTx(someone, abi.HipoFinanceReserveTokensMsgOp),
			),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, WithdrawHipoStakeRequestStraw.Merge(tt.bubble))
			if !tt.want {
				return
			}
			request, ok := tt.bubble.Info.(BubbleWithdrawStakeRequest)
			require.True(t, ok)
			require.Equal(t, core.StakingImplementationHipo, request.Implementation)
			require.Equal(t, tt.success, request.Success)
		})
	}
}

// A bill of a finished round settles in one of three ways, and each has to stay visible:
// paid out, postponed to a later round, or - when no round is left - handed back.
func TestHipoBillSettlement(t *testing.T) {
	var someone tongo.AccountID
	paidOut := hipoSettlement(
		hipoTx(references.HipoParent, abi.HipoFinanceProxyTokensBurnedMsgOp,
			hipoTx(someone, abi.HipoFinanceTokensBurnedMsgOp,
				hipoTx(someone, abi.HipoFinanceWithdrawalNotificationMsgOp),
			),
		),
	)
	require.True(t, WithdrawHipoStakeSettledStraw.Merge(paidOut))
	require.IsType(t, BubbleWithdrawStake{}, paidOut.Info)

	postponed := hipoSettlement(
		hipoTx(someone, abi.HipoFinanceMintBillMsgOp,
			hipoTx(someone, abi.HipoFinanceAssignBillMsgOp),
		),
	)
	require.False(t, WithdrawHipoStakeSettledStraw.Merge(postponed))
	require.True(t, WithdrawHipoStakePostponedStraw.Merge(postponed))
	require.IsType(t, BubbleWithdrawStakeRequest{}, postponed.Info)

	// The rollback leg is recognized on its own, wherever the treasury gives up, so it
	// reports the hGRAM returning instead of leaving the burn above it unanswered.
	rolledBack := hipoSettlement(
		hipoFrom(hipoTx(references.HipoParent, abi.HipoFinanceProxyRollbackUnstakeMsgOp,
			hipoTx(someone, abi.HipoFinanceRollbackUnstakeMsgOp),
		), references.HipoTreasury),
	)
	require.False(t, WithdrawHipoStakeSettledStraw.Merge(rolledBack))
	require.False(t, WithdrawHipoStakePostponedStraw.Merge(rolledBack))
	require.True(t, JettonMintHipoRollbackStraw.Merge(rolledBack.Children[0]))
	require.IsType(t, BubbleJettonMint{}, rolledBack.Children[0].Info)
}

// The mint has to be recognized on its own, because it is the only place the hGRAM figure
// appears: at the end of an instant stake and, rounds later, when a deferred one settles.
func TestJettonMintHipoStraw(t *testing.T) {
	var someone tongo.AccountID
	mint := func(from tongo.AccountID) *Bubble {
		b := hipoTx(someone, abi.HipoFinanceTokensMintedMsgOp,
			hipoTx(someone, abi.JettonNotifyMsgOp),
		)
		tx := b.Info.(BubbleTx)
		tx.inputFrom = &Account{Address: from}
		b.Info = tx
		return b
	}
	fromParent := mint(references.HipoParent)
	require.True(t, JettonMintHipoStraw.Merge(fromParent))
	require.IsType(t, BubbleJettonMint{}, fromParent.Info)
	require.Equal(t, references.HipoParent, fromParent.Info.(BubbleJettonMint).master)

	// Anyone may send the op; only the jetton master's copy mints hGRAM.
	require.False(t, JettonMintHipoStraw.Merge(mint(someone)))
}

// hipoFrom marks who sent the message a bubble is handling.
func hipoFrom(b *Bubble, sender tongo.AccountID) *Bubble {
	tx := b.Info.(BubbleTx)
	tx.inputFrom = &Account{Address: sender}
	b.Info = tx
	return b
}

// Everything about an hGRAM balance is derived from messages that anyone may send, so each
// straw that moves the jetton flow has to be anchored to an address nobody else can speak
// from. These are the forgeries a scammer would reach for.
func TestHipoStrawsRejectForgeries(t *testing.T) {
	attacker := tongo.MustParseAccountID("0:1111111111111111111111111111111111111111111111111111111111111111")

	// A contract of the attacker's plays the jetton master and hands hGRAM to a stranger.
	t.Run("mint from an impostor master", func(t *testing.T) {
		forged := hipoFrom(hipoTx(attacker, abi.HipoFinanceTokensMintedMsgOp), attacker)
		require.False(t, JettonMintHipoStraw.Merge(forged))
	})

	// The same credit, through the rollback leg, which names its own recipient.
	t.Run("rollback the treasury never sent", func(t *testing.T) {
		forged := hipoFrom(
			hipoTx(attacker, abi.HipoFinanceProxyRollbackUnstakeMsgOp,
				hipoTx(attacker, abi.HipoFinanceRollbackUnstakeMsgOp),
			), attacker)
		require.False(t, JettonMintHipoRollbackStraw.Merge(forged))
	})

	// And the shape that matters, because the treasury really does send it: reserve_tokens
	// authenticates nobody (contracts/treasury.fc), so anyone may send it one naming any
	// owner, and it answers by returning proxy_rollback_unstake TO THE SENDER with that
	// owner copied across. Coming from the treasury is therefore not evidence of anything;
	// only arriving at the jetton master is.
	t.Run("rollback the treasury sent to an impostor", func(t *testing.T) {
		forged := hipoFrom(
			hipoTx(attacker, abi.HipoFinanceProxyRollbackUnstakeMsgOp,
				hipoTx(attacker, abi.HipoFinanceRollbackUnstakeMsgOp),
			), references.HipoTreasury)
		require.False(t, JettonMintHipoRollbackStraw.Merge(forged))
	})

	// The genuine one, for contrast: the treasury sends it to the jetton master.
	t.Run("genuine rollback still mints", func(t *testing.T) {
		real := hipoFrom(
			hipoTx(references.HipoParent, abi.HipoFinanceProxyRollbackUnstakeMsgOp,
				hipoTx(attacker, abi.HipoFinanceRollbackUnstakeMsgOp),
			), references.HipoTreasury)
		require.True(t, JettonMintHipoRollbackStraw.Merge(real))
	})

	// And in the debit direction: reserve_tokens really can be sent to the treasury by
	// anyone (mainnet tx 4de4a490e0d9b523...), which answers it with a rollback. Wrapped
	// in a proxy_reserve_tokens of the attacker's own, that would book hGRAM out of
	// whichever holder they named.
	t.Run("unstake that bypassed the jetton master", func(t *testing.T) {
		forged := hipoFrom(
			hipoTx(attacker, abi.HipoFinanceProxyReserveTokensMsgOp,
				hipoFrom(hipoTx(references.HipoTreasury, abi.HipoFinanceReserveTokensMsgOp,
					hipoTx(attacker, abi.HipoFinanceProxyRollbackUnstakeMsgOp),
				), attacker),
			), attacker)
		require.False(t, WithdrawHipoStakeRequestStraw.Merge(forged))
	})
}
