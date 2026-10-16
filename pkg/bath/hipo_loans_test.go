package bath

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
)

// Real logs, the same ones pkg/core's parser tests read.
const (
	hipoLoanLogBody      = "b5ee9c720101010100460000876a9e4f0850ba43b740007077038d7ea4c68000708f0db1157feef523aff0d8e07fff801a6d1cefedb5ca6bcc88f0a7300745ee3e0fbaf722aa9040af454ee47dc2d130d0"
	hipoRepaymentLogBody = "b5ee9c7201010101004b0000916a944f08707268034320633703328b944c4000703f288f2fa449a601375194ce1c05344a2f57fd8007b591d71001a927c6fc6b00e107ffea04e4a60f955f4f6c6475b5d6c02703bcb0"
	hipoFinishLogBody    = "b5ee9c720101010100240000446a944f0870e4a290e8d093470e4c97b1b6a56c71c940a9e703cb07189eaf16dfb68a"
	// when each was written: the loan after the 2026-09-05 release, the repayment before it
	hipoLoanLogAt      = 1788761000
	hipoRepaymentLogAt = 1788200000
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

// hipoLogMsg builds an external-out message the way emit_log does: the topic in a 256-bit
// external destination, the body from the real log of that topic.
func hipoLogMsg(t *testing.T, topic core.HipoLogTopic, lt uint64, body string) core.Message {
	at := uint32(hipoLoanLogAt)
	if topic == core.HipoLogRepayment {
		at = hipoRepaymentLogAt
	}
	t.Helper()
	dest := boc.NewBitString(256)
	require.NoError(t, dest.WriteBigUint(new(big.Int).SetUint64(uint64(topic)), 256))
	return core.Message{
		MessageID:         core.MessageID{CreatedLt: lt},
		CreatedAt:         at,
		MsgType:           core.ExtOutMsg,
		DestinationExtern: &dest,
		Body:              mustHex(t, body),
	}
}

func hipoLogged(b *Bubble, logs ...core.Message) *Bubble {
	tx := b.Info.(BubbleTx)
	tx.externalOut = logs
	b.Info = tx
	return b
}

// hipoSent marks the message a bubble handles as sent at lt by a transaction that logged logs.
func hipoSent(b *Bubble, lt uint64, logs ...core.Message) *Bubble {
	tx := b.Info.(BubbleTx)
	tx.inputLt = lt
	tx.senderExternalOut = logs
	b.Info = tx
	return b
}

func hipoExternal(b *Bubble, body any) *Bubble {
	tx := b.Info.(BubbleTx)
	tx.external = true
	tx.decodedBody.Value = body
	b.Info = tx
	return b
}

func TestHipoLoanStraws(t *testing.T) {
	loan := tongo.MustParseAccountID("-1:1e530983ee3f8be8529cc6c03199953f9fe9b46f7845219f78e888968e9c54ea")

	t.Run("a loan is issued from the log written next to its message", func(t *testing.T) {
		issue := hipoSent(hipoFrom(hipoTx(loan, abi.HipoFinanceProxyNewStakeMsgOp), references.HipoTreasury), 100,
			hipoLogMsg(t, core.HipoLogLoan, 99, hipoLoanLogBody),
			hipoLogMsg(t, core.HipoLogLoan, 101, hipoLoanLogBody),
		)
		require.True(t, LoanIssueHipoStraw.Merge(issue))
		issued := issue.Info.(BubbleLoanIssue)
		want, err := core.ParseHipoLoanLog(mustHex(t, hipoLoanLogBody), hipoLoanLogAt)
		require.NoError(t, err)
		require.Equal(t, int64(want.LoanAmount), issued.Amount)
		require.Equal(t, loan, issued.Loan)
		require.Equal(t, references.HipoTreasury, issued.Pool)
		require.Contains(t, issue.Accounts, issued.Borrower, "the borrower sees the loan in their history")
	})

	t.Run("a loan with no log of its own is not issued", func(t *testing.T) {
		issue := hipoSent(hipoFrom(hipoTx(loan, abi.HipoFinanceProxyNewStakeMsgOp), references.HipoTreasury), 100,
			hipoLogMsg(t, core.HipoLogLoan, 99, hipoLoanLogBody),
		)
		require.False(t, LoanIssueHipoStraw.Merge(issue))
	})

	t.Run("a repayment is split by its log", func(t *testing.T) {
		result := hipoLogged(hipoFrom(hipoTx(references.HipoTreasury, abi.HipoFinanceRecoverStakeResultMsgOp,
			hipoFrom(hipoTx(loan, abi.HipoFinanceLoanResultMsgOp), references.HipoTreasury),
		), loan), hipoLogMsg(t, core.HipoLogRepayment, 1, hipoRepaymentLogBody))
		require.True(t, LoanRepaymentHipoStraw.Merge(result))
		repaid := result.Info.(BubbleLoanRepayment)
		want, err := core.ParseHipoRepaymentLog(mustHex(t, hipoRepaymentLogBody), hipoRepaymentLogAt)
		require.NoError(t, err)
		require.Equal(t, int64(want.StakersShare), repaid.PoolReward)
		require.Equal(t, int64(want.BorrowerShare), repaid.BorrowerPayout)
		require.Equal(t, loan, repaid.Loan)
		require.Empty(t, result.Children, "loan_result is part of the repayment")
	})

	t.Run("the settled round carries the round's totals", func(t *testing.T) {
		collection := tongo.MustParseAccountID("0:7e40f97d00000000000000000000000000000000000000000000000000000000")
		burn := hipoFrom(hipoTx(collection, abi.HipoFinanceBurnAllMsgOp,
			hipoLogged(hipoTx(references.HipoTreasury, abi.HipoFinanceLastBillBurnedMsgOp),
				hipoLogMsg(t, core.HipoLogFinish, 1, hipoFinishLogBody)),
		), references.HipoTreasury)
		require.True(t, PoolRoundSettledHipoStraw.Merge(burn))
		settled := burn.Info.(BubblePoolRoundTransition)
		want := core.HipoFinishLog{}
		require.NoError(t, core.ParseHipoLog(mustHex(t, hipoFinishLogBody), &want))
		require.Equal(t, PoolRoundSettled, settled.Transition)
		require.Equal(t, want.RoundSince, settled.RoundSince)
		require.Equal(t, int64(want.TotalRecovered), *settled.TotalRecovered)
	})

	t.Run("the round folds in the treasury's own steps", func(t *testing.T) {
		round := hipoExternal(hipoTx(references.HipoTreasury, abi.HipoFinanceParticipateInElectionExtInMsgOp,
			hipoFrom(hipoTx(references.HipoTreasury, abi.HipoFinanceDecideLoanRequestsMsgOp,
				hipoFrom(hipoTx(references.HipoTreasury, abi.HipoFinanceProcessLoanRequestsMsgOp,
					hipoFrom(hipoTx(loan, abi.HipoFinanceProxyNewStakeMsgOp), references.HipoTreasury),
				), references.HipoTreasury),
			), references.HipoTreasury),
		), abi.HipoFinanceParticipateInElectionExtInMsgBody{RoundSince: 1792102152})
		MergeAllBubbles(round, []Merger{PoolRoundElectionHipoStraw, hipoRoundStepStraw})
		election := round.Info.(BubblePoolRoundTransition)
		require.Equal(t, PoolRoundElection, election.Transition)
		require.Equal(t, uint32(1792102152), election.RoundSince)
		require.Len(t, round.Children, 1, "only the loan is left beside the round")
	})
}

// Every loan action names a borrower and an amount, so each is anchored on something only
// the treasury can produce: its own logs, or its own transaction.
func TestHipoLoanStrawsRejectForgeries(t *testing.T) {
	attacker := tongo.MustParseAccountID("0:1111111111111111111111111111111111111111111111111111111111111111")

	t.Run("a log next to a message the treasury did not send", func(t *testing.T) {
		forged := hipoSent(hipoFrom(hipoTx(attacker, abi.HipoFinanceProxyNewStakeMsgOp), attacker), 100,
			hipoLogMsg(t, core.HipoLogLoan, 101, hipoLoanLogBody))
		require.False(t, LoanIssueHipoStraw.Merge(forged))
	})
	t.Run("a repayment log from another contract", func(t *testing.T) {
		forged := hipoLogged(hipoFrom(hipoTx(attacker, abi.HipoFinanceRecoverStakeResultMsgOp), attacker),
			hipoLogMsg(t, core.HipoLogRepayment, 1, hipoRepaymentLogBody))
		require.False(t, LoanRepaymentHipoStraw.Merge(forged))
	})
	t.Run("a recover_stake_result the treasury did not log", func(t *testing.T) {
		forged := hipoFrom(hipoTx(references.HipoTreasury, abi.HipoFinanceRecoverStakeResultMsgOp), attacker)
		require.False(t, LoanRepaymentHipoStraw.Merge(forged))
	})
	t.Run("a round trigger at another contract", func(t *testing.T) {
		forged := hipoExternal(hipoTx(attacker, abi.HipoFinanceFinishParticipationExtInMsgOp),
			abi.HipoFinanceFinishParticipationExtInMsgBody{})
		require.False(t, PoolRoundEndedHipoStraw.Merge(forged))
	})
	t.Run("a burn_all the treasury did not send", func(t *testing.T) {
		forged := hipoFrom(hipoTx(attacker, abi.HipoFinanceBurnAllMsgOp), attacker)
		require.False(t, PoolRoundSettledHipoStraw.Merge(forged))
	})
	t.Run("a loan request at another contract", func(t *testing.T) {
		forged := hipoFrom(hipoTx(attacker, abi.HipoFinanceRequestLoanV4MsgOp), attacker)
		require.False(t, LoanRequestHipoStraw.Merge(forged))
	})
}
