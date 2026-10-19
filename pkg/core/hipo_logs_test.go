package core

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

// Real logs, taken from the treasury's own transactions on mainnet and identified by the
// topic in their destination address. Each is checked field by field against the log_*
// functions in the Hipo contracts, and ParseHipoLog additionally insists the body was
// consumed exactly - which is what would catch a layout drifting from the contract.
const (
	hipoLoanLogBody      = "b5ee9c720101010100460000876a9e4f0850ba43b740007077038d7ea4c68000708f0db1157feef523aff0d8e07fff801a6d1cefedb5ca6bcc88f0a7300745ee3e0fbaf722aa9040af454ee47dc2d130d0"
	hipoRepaymentLogBody = "b5ee9c7201010101004b0000916a944f08707268034320633703328b944c4000703f288f2fa449a601375194ce1c05344a2f57fd8007b591d71001a927c6fc6b00e107ffea04e4a60f955f4f6c6475b5d6c02703bcb0"
	hipoFinishLogBody    = "b5ee9c720101010100240000446a944f0870e4a290e8d093470e4c97b1b6a56c71c940a9e703cb07189eaf16dfb68a"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

func TestHipoParseLoanLog(t *testing.T) {
	log, err := ParseHipoLoanLog(mustHex(t, hipoLoanLogBody), hipoBorrowerFeeRelease+1)
	require.NoError(t, err)

	require.Equal(t, uint32(1788759816), log.RoundSince)
	// A rate out of 65535, not an amount: 1799 is the share this borrower bid.
	require.Equal(t, uint16(1799), log.BorrowerRewardShare)
	require.EqualValues(t, "AddrStd", log.Borrower.SumType)
	// The loan is the pool's money; the stake is the borrower's own, posted alongside it.
	require.Greater(t, uint64(log.LoanAmount), uint64(0))
	require.Greater(t, uint64(log.StakeAmount), uint64(0))
	fmt.Printf("loan log: round=%d min_payment=%d share=%d loan=%d accrue=%d stake=%d fee=%d\n",
		log.RoundSince, log.MinPayment, log.BorrowerRewardShare, log.LoanAmount,
		log.AccrueAmount, log.StakeAmount, log.BorrowerFee)
}

func TestHipoParseRepaymentLog(t *testing.T) {
	// Recovered before 2026-09-05, so it is in the older six-amount layout and BurnShare
	// is legitimately zero.
	log, err := ParseHipoRepaymentLog(mustHex(t, hipoRepaymentLogBody), hipoBorrowerFeeRelease-1)
	require.NoError(t, err)
	require.Zero(t, uint64(log.BurnShare))

	require.Equal(t, uint32(1788104456), log.RoundSince)
	require.EqualValues(t, "AddrStd", log.Borrower.SumType)
	// This is the whole reason the logs are read: the split exists in no message.
	require.Greater(t, uint64(log.RepaymentAmount), uint64(0))
	require.Greater(t, uint64(log.StakersShare), uint64(0))
	// A repayment covers the loan and then some, and what it pays out cannot exceed it.
	require.GreaterOrEqual(t, uint64(log.RepaymentAmount), uint64(log.LoanAmount))
	split := uint64(log.StakersShare) + uint64(log.GovernorShare) +
		uint64(log.BorrowerShare) + uint64(log.BurnShare)
	require.LessOrEqual(t, split, uint64(log.RepaymentAmount))
	fmt.Printf("repayment log: round=%d repaid=%d loan=%d accrue=%d stakers=%d governor=%d borrower=%d burn=%d\n",
		log.RoundSince, log.RepaymentAmount, log.LoanAmount, log.AccrueAmount,
		log.StakersShare, log.GovernorShare, log.BorrowerShare, log.BurnShare)
}

func TestHipoParseFinishLog(t *testing.T) {
	var log HipoFinishLog
	require.NoError(t, ParseHipoLog(mustHex(t, hipoFinishLogBody), &log))

	require.Equal(t, uint32(1788104456), log.RoundSince)
	// total_coins and total_tokens are the exchange rate at the moment the round finished,
	// so both have to be there for the rate to be derivable.
	require.Greater(t, uint64(log.TotalCoins), uint64(0))
	require.Greater(t, uint64(log.TotalTokens), uint64(0))
	// A round that was lent out and recovered more than it staked is the normal case.
	require.GreaterOrEqual(t, uint64(log.TotalRecovered), uint64(log.TotalStaked))
	fmt.Printf("finish log: round=%d staked=%d recovered=%d total_coins=%d total_tokens=%d rate=%.6f\n",
		log.RoundSince, log.TotalStaked, log.TotalRecovered, log.TotalCoins, log.TotalTokens,
		float64(log.TotalCoins)/float64(log.TotalTokens))
}

// A body that is one field short of its layout must be refused rather than half-read.
func TestHipoParseLogRejectsShortBody(t *testing.T) {
	var log HipoFinishLog
	require.Error(t, ParseHipoLog(mustHex(t, hipoRepaymentLogBody), &log),
		"a repayment body read as a finish log leaves bits over and must not be accepted")
}

// Every treasury log in the replay corpus, parsed. The point is not that a few samples
// work: it is that no body anywhere is left half-read, since a half-read body is exactly
// what a layout that has drifted from the contract looks like.
//
// Both logs changed shape with the 2026-09-05 borrower-fee release, and the corpus spans
// it, so this also covers the fallback to the older layouts.
func TestHipoParseAllLogsInCorpus(t *testing.T) {
	dir := os.Getenv("REPLAY_DIR")
	if dir == "" {
		t.Skip("set REPLAY_DIR to the directory holding log_samples.json")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "log_samples.json"))
	if err != nil {
		t.Skipf("no corpus: %v", err)
	}
	byTopic := map[string][]string{}
	require.NoError(t, json.Unmarshal(raw, &byTopic))

	counts := map[string]int{}
	for topic, bodies := range byTopic {
		for _, b := range bodies {
			body := mustHex(t, b)
			switch topic[len(topic)-1] {
			case '1':
				// the samples carry no time, so either era may be the right one
				log, err := ParseHipoLoanLog(body, hipoBorrowerFeeRelease)
				if err != nil {
					log, err = ParseHipoLoanLog(body, 1)
				}
				require.NoErrorf(t, err, "loan log %s", b)
				require.NotZero(t, log.RoundSince)
				require.LessOrEqual(t, uint64(log.BorrowerRewardShare), uint64(65535))
			case '2':
				log, err := ParseHipoRepaymentLog(body, hipoBorrowerFeeRelease)
				if err != nil {
					log, err = ParseHipoRepaymentLog(body, 1)
				}
				require.NoErrorf(t, err, "repayment log %s", b)
				require.NotZero(t, log.RoundSince)
				split := uint64(log.StakersShare) + uint64(log.GovernorShare) +
					uint64(log.BorrowerShare) + uint64(log.BurnShare)
				require.LessOrEqualf(t, split, uint64(log.RepaymentAmount),
					"a repayment cannot pay out more than it brought in: %s", b)
			case '3':
				var log HipoFinishLog
				require.NoErrorf(t, ParseHipoLog(body, &log), "finish log %s", b)
				require.NotZero(t, log.TotalTokens)
			default:
				t.Fatalf("unexpected topic %s", topic)
			}
			counts[topic]++
		}
	}
	total := 0
	for topic, n := range counts {
		fmt.Printf("topic ...%s: %d logs parsed, none left half-read\n", topic[len(topic)-4:], n)
		total += n
	}
	require.Greater(t, total, 100, "corpus should not have shrunk")
}

// The trace view shows a log decoded only when the treasury wrote it: a topic is an address
// any contract can write to.
func TestHipoLogsDecodeOnlyFromTheTreasury(t *testing.T) {
	topic := boc.NewBitString(256)
	require.NoError(t, topic.WriteBigUint(big.NewInt(int64(HipoLogRepayment)), 256))
	body := mustHex(t, hipoRepaymentLogBody)

	treasury := references.HipoTreasury
	decoded, ok := decodeHipoLog(&treasury, &topic, body, hipoBorrowerFeeRelease-1)
	require.True(t, ok)
	require.Equal(t, "HipoFinanceLogRepayment", decoded.Operation)
	require.IsType(t, HipoRepaymentLog{}, decoded.Value)

	other := ton.MustParseAccountID("0:1111111111111111111111111111111111111111111111111111111111111111")
	_, ok = decodeHipoLog(&other, &topic, body, hipoBorrowerFeeRelease-1)
	require.False(t, ok)

	// A body of another topic's layout is left undecoded rather than half-read.
	finish := boc.NewBitString(256)
	require.NoError(t, finish.WriteBigUint(big.NewInt(int64(HipoLogFinish)), 256))
	_, ok = decodeHipoLog(&treasury, &finish, body, hipoBorrowerFeeRelease-1)
	require.False(t, ok)
}

// An old loan log can fit the new layout exactly and read as nonsense, so the layout is
// chosen by when the log was written. The figures are the reviewer's counterexample.
func TestHipoLoanLogEraIsByTime(t *testing.T) {
	borrower := ton.MustParseAccountID("0:7a228a801e58bdf435a39c93c8efb5a3736b58455381c04c7322cda3512b5bd4")
	old := hipoLoanLogV1{
		RoundSince:          1788000000,
		MinPayment:          544807109759,
		BorrowerRewardShare: 17,
		LoanAmount:          1662881220370992,
		AccrueAmount:        124844006369212,
		StakeAmount:         1486722724167,
		Borrower:            borrower.ToMsgAddress(),
	}
	cell := boc.NewCell()
	require.NoError(t, tlb.Marshal(cell, old))
	body, err := cell.ToBoc()
	require.NoError(t, err)

	var misread HipoLoanLog
	require.NoError(t, ParseHipoLog(body, &misread), "the old body fits the new layout too")
	require.NotEqual(t, old.LoanAmount, misread.LoanAmount)

	log, err := ParseHipoLoanLog(body, hipoBorrowerFeeRelease-1)
	require.NoError(t, err)
	require.Equal(t, old.LoanAmount, log.LoanAmount)
	// a log of unknown time is taken as current
	unknown, err := ParseHipoLoanLog(body, 0)
	require.NoError(t, err)
	require.Equal(t, misread, unknown)
	require.Equal(t, old.AccrueAmount, log.AccrueAmount)
	require.Equal(t, uint16(17*257), log.BorrowerRewardShare)
}

// The burner logs by topic too. A real swap log, written when the burner sold 15.647 hGRAM.
func TestHipoBurnerLogsDecodeOnlyFromTheBurner(t *testing.T) {
	topic := boc.NewBitString(256)
	require.NoError(t, topic.WriteBigUint(big.NewInt(3), 256))
	body := mustHex(t, "b5ee9c720101010100160000280000000000000185503a4a5123e60154ee22f17f")

	burner := references.HipoBurner
	decoded, ok := decodeHipoBurnerLog(&burner, &topic, body)
	require.True(t, ok)
	require.Equal(t, "HipoBurnerLogSwap", decoded.Operation)
	require.Equal(t, HipoBurnerSwapLog{QueryId: 389, Amount: 15647183422, TotalSwapped: 1464284148095}, decoded.Value)

	treasury := references.HipoTreasury
	_, ok = decodeHipoBurnerLog(&treasury, &topic, body)
	require.False(t, ok)
}

func TestHipoDecodeLog(t *testing.T) {
	topic := boc.NewBitString(256)
	require.NoError(t, topic.WriteBigUint(big.NewInt(int64(HipoLogRepayment)), 256))
	msg := Message{MsgType: ExtOutMsg, DestinationExtern: &topic, Body: mustHex(t, hipoRepaymentLogBody), CreatedAt: hipoBorrowerFeeRelease - 1}

	t.Run("parses the body when nothing decoded it", func(t *testing.T) {
		got, value, ok := DecodeHipoLog(msg)
		require.True(t, ok)
		require.Equal(t, HipoLogRepayment, got)
		require.IsType(t, HipoRepaymentLog{}, value)
	})
	t.Run("reuses the decoded body", func(t *testing.T) {
		decoded := HipoRepaymentLog{RoundSince: 42}
		reused := msg
		reused.Body = []byte{1, 2, 3}
		reused.DecodedBody = &DecodedMessageBody{Operation: "HipoFinanceLogRepayment", Value: decoded}
		_, value, ok := DecodeHipoLog(reused)
		require.True(t, ok)
		require.Equal(t, decoded, value)
	})
	t.Run("a body that does not fit is refused", func(t *testing.T) {
		unknownTime := msg
		unknownTime.CreatedAt = 0
		_, _, ok := DecodeHipoLog(unknownTime)
		require.False(t, ok)
	})
}
