package core

import (
	"fmt"
	"reflect"

	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

// Hipo's treasury reports what a round did in external-out messages rather than in the
// messages it sends, and the two carry different things. reserve_tokens, loan_result and
// recover_stake_result carry a query id and an address; how much a loan repaid, and how
// that repayment was split between stakers, the governor, the borrower and the burner,
// exists only here.
//
// The shape is unusual enough to be worth stating. emit_log puts the topic in the message's
// destination - a 256-bit external address holding a small integer - and starts the body at
// round_since with no op-code at all. Decoders that key external-out messages on a body
// op-code therefore read round_since as one, and report a different op-code for the same
// log every round. These are parsed by topic instead.
//
// Layouts follow log_loan, log_repayment and log_finish in contracts/imports/utils.fc, and
// log_deficit in treasury.fc. Amounts are Coins; borrower_reward_share and borrower_fee are
// rates out of 65535, not amounts.

const (
	HipoLogLoan                HipoLogTopic = 1
	HipoLogRepayment           HipoLogTopic = 2
	HipoLogFinish              HipoLogTopic = 3
	HipoLogFailedBurningTokens HipoLogTopic = 4
	HipoLogDeficit             HipoLogTopic = 5
)

// Two of these logs changed shape with Hipo's 2026-09-05 release, which introduced the
// borrower fee. Both layouts are parsed, because actions are recomputed from history and a
// round settled before that release would otherwise be unreadable. The layout is chosen by
// when the log was written, not by which one fits: an old body can fit the new layout
// exactly and read as nonsense, and 8 of the 542 real logs around the release fit both.
// hipoBorrowerFeeRelease is the treasury's upgrade transaction (9722cf28d0be9a08...) on
// 2026-09-05 at 17:29:33 UTC; every log written after it is in the new layout.
const hipoBorrowerFeeRelease = 1788629373

// each log is named the way a decoded message is named
var hipoLogOperations = map[HipoLogTopic]string{
	HipoLogLoan:                "HipoFinanceLogLoan",
	HipoLogRepayment:           "HipoFinanceLogRepayment",
	HipoLogFinish:              "HipoFinanceLogFinish",
	HipoLogFailedBurningTokens: "HipoFinanceLogFailedBurningTokens",
	HipoLogDeficit:             "HipoFinanceLogDeficit",
}

var hipoBurnerLogs = map[uint64]struct {
	operation string
	layout    func() any
}{
	1: {"HipoBurnerLogReceived", func() any { return &HipoBurnerReceivedLog{} }},
	2: {"HipoBurnerLogDeposit", func() any { return &HipoBurnerDepositLog{} }},
	3: {"HipoBurnerLogSwap", func() any { return &HipoBurnerSwapLog{} }},
	4: {"HipoBurnerLogBurn", func() any { return &HipoBurnerBurnLog{} }},
	5: {"HipoBurnerLogDiscovery", func() any { return &HipoBurnerDiscoveryLog{} }},
	6: {"HipoBurnerLogUpgrade", func() any { return &HipoBurnerUpgradeLog{} }},
}

type HipoLogTopic uint64

// HipoLoanLog is emitted once per loan the treasury hands out, as the round is distributed.
type HipoLoanLog struct {
	RoundSince uint32
	MinPayment tlb.Grams
	// BorrowerRewardShare is always scaled to 65535 here. Before 2026-09-05 the field was 8
	// bits out of 255, and is converted on the way out by the factor the contract's own
	// migration note gives: 255 * 257 = 65535.
	BorrowerRewardShare uint16
	LoanAmount          tlb.Grams
	AccrueAmount        tlb.Grams
	StakeAmount         tlb.Grams
	// BorrowerFee is a rate out of 65535, snapshotted from the extension when the request
	// was made so that set_borrower_fee cannot reprice a committed loan. Zero for a loan
	// made before the fee existed, which is accurate rather than missing.
	BorrowerFee uint16
	Borrower    tlb.MsgAddress
}

// HipoRepaymentLog is emitted once per loan recovered, and is the only place the split of a
// repayment appears. StakersShare is what reaches the exchange rate.
type HipoRepaymentLog struct {
	RoundSince      uint32
	RepaymentAmount tlb.Grams
	LoanAmount      tlb.Grams
	AccrueAmount    tlb.Grams
	StakersShare    tlb.Grams
	GovernorShare   tlb.Grams
	BorrowerShare   tlb.Grams
	// BurnShare is zero for a loan recovered before 2026-09-05, when no share was burned.
	BurnShare tlb.Grams
	Borrower  tlb.MsgAddress
}

// HipoFinishLog is emitted once per round, when its last loan has been recovered.
type HipoFinishLog struct {
	RoundSince     uint32
	TotalStaked    tlb.Grams
	TotalRecovered tlb.Grams
	TotalCoins     tlb.Grams
	TotalTokens    tlb.Grams
}

// HipoFailedBurningLog is emitted when a bill burns that the treasury cannot settle.
type HipoFailedBurningLog struct {
	RoundSince  uint32
	TotalCoins  tlb.Grams
	TotalTokens tlb.Grams
	Coins       tlb.Grams
	Tokens      tlb.Grams
	Owner       tlb.MsgAddress
}

// HipoDeficitLog is emitted alongside a repayment that did not cover what it owed.
type HipoDeficitLog struct {
	RoundSince   uint32
	LoanAmount   tlb.Grams
	AccrueAmount tlb.Grams
	IncomingTon  tlb.Grams
	Shortfall    tlb.Grams
	Deficit      tlb.Grams
	Borrower     tlb.MsgAddress
}

// Hipo's burner logs the same way, with topics of its own (contracts/burner.fc in
// HipoFinance/burner). Each log is read only from the burner.
type HipoBurnerReceivedLog struct {
	Op            uint32
	Value         tlb.Grams
	TotalReceived tlb.Grams
	Source        tlb.MsgAddress
}

type HipoBurnerDepositLog struct {
	QueryId uint64
	Stake   tlb.Grams
}

type HipoBurnerSwapLog struct {
	QueryId      uint64
	Amount       tlb.Grams
	TotalSwapped tlb.Grams
}

type HipoBurnerBurnLog struct {
	QueryId     uint64
	Amount      tlb.Grams
	TotalBurned tlb.Grams
}

type HipoBurnerDiscoveryLog struct {
	QueryId uint64
}

type HipoBurnerUpgradeLog struct {
	QueryId uint64
	Source  tlb.MsgAddress
}

// the pre-2026-09-05 shapes, kept only so that history still parses
type hipoLoanLogV1 struct {
	RoundSince          uint32
	MinPayment          tlb.Grams
	BorrowerRewardShare uint8
	LoanAmount          tlb.Grams
	AccrueAmount        tlb.Grams
	StakeAmount         tlb.Grams
	Borrower            tlb.MsgAddress
}

type hipoRepaymentLogV1 struct {
	RoundSince      uint32
	RepaymentAmount tlb.Grams
	LoanAmount      tlb.Grams
	AccrueAmount    tlb.Grams
	StakersShare    tlb.Grams
	GovernorShare   tlb.Grams
	BorrowerShare   tlb.Grams
	Borrower        tlb.MsgAddress
}

func ParseHipoLoanLog(body []byte, createdAt uint32) (HipoLoanLog, error) {
	if hipoCurrentLayout(createdAt) {
		var log HipoLoanLog
		err := ParseHipoLog(body, &log)
		return log, err
	}
	var old hipoLoanLogV1
	if err := ParseHipoLog(body, &old); err != nil {
		return HipoLoanLog{}, err
	}
	return HipoLoanLog{
		RoundSince: old.RoundSince,
		MinPayment: old.MinPayment,
		// 255 -> 65535, the conversion the contract's own note prescribes.
		BorrowerRewardShare: uint16(old.BorrowerRewardShare) * 257,
		LoanAmount:          old.LoanAmount,
		AccrueAmount:        old.AccrueAmount,
		StakeAmount:         old.StakeAmount,
		Borrower:            old.Borrower,
	}, nil
}

func ParseHipoRepaymentLog(body []byte, createdAt uint32) (HipoRepaymentLog, error) {
	if hipoCurrentLayout(createdAt) {
		var log HipoRepaymentLog
		err := ParseHipoLog(body, &log)
		return log, err
	}
	var old hipoRepaymentLogV1
	if err := ParseHipoLog(body, &old); err != nil {
		return HipoRepaymentLog{}, err
	}
	return HipoRepaymentLog{
		RoundSince:      old.RoundSince,
		RepaymentAmount: old.RepaymentAmount,
		LoanAmount:      old.LoanAmount,
		AccrueAmount:    old.AccrueAmount,
		StakersShare:    old.StakersShare,
		GovernorShare:   old.GovernorShare,
		BorrowerShare:   old.BorrowerShare,
		Borrower:        old.Borrower,
	}, nil
}

func HipoLogTopicOf(msg Message) (HipoLogTopic, bool) {
	// The topic is just an address any contract can write to: callers must check that the
	// treasury wrote the message.
	if msg.MsgType != ExtOutMsg {
		return 0, false
	}
	return hipoTopic(msg.DestinationExtern)
}

func DecodeHipoLog(msg Message) (HipoLogTopic, any, bool) {
	topic, ok := HipoLogTopicOf(msg)
	if !ok {
		return 0, nil, false
	}
	// ConvertMessage has already decoded the logs of the treasury it saw; a store may not have.
	if msg.DecodedBody != nil && msg.DecodedBody.Operation == hipoLogOperations[topic] {
		switch msg.DecodedBody.Value.(type) {
		case HipoLoanLog, HipoRepaymentLog, HipoFinishLog, HipoFailedBurningLog, HipoDeficitLog:
			return topic, msg.DecodedBody.Value, true
		}
	}
	value, err := parseHipoLog(topic, msg.Body, msg.CreatedAt)
	if err != nil {
		return 0, nil, false
	}
	return topic, value, true
}

func ParseHipoLog(body []byte, dst any) error {
	if len(body) == 0 {
		return fmt.Errorf("empty body")
	}
	cells, err := boc.DeserializeBoc(body)
	if err != nil {
		return err
	}
	if len(cells) != 1 {
		return fmt.Errorf("expected one root cell, got %d", len(cells))
	}
	if err := tlb.Unmarshal(cells[0], dst); err != nil {
		return err
	}
	// The body must be consumed exactly. That is the only defence against a layout that has
	// drifted from the contract: a field read at the wrong width still decodes and still
	// yields a plausible number, and nothing but the leftover bits says otherwise.
	if left := cells[0].BitsAvailableForRead(); left != 0 {
		return fmt.Errorf("%d bits left unread: the layout does not match the contract", left)
	}
	return nil
}

func decodeHipoLog(source *ton.AccountID, dest *ExternalAddress, body []byte, createdAt uint32) (*DecodedMessageBody, bool) {
	// abi cannot decode these: it dispatches on a body op-code, and these bodies start at
	// round_since. Only the treasury's own messages are read.
	if source == nil || *source != references.HipoTreasury {
		return nil, false
	}
	topic, ok := hipoTopic(dest)
	if !ok {
		return nil, false
	}
	value, err := parseHipoLog(topic, body, createdAt)
	if err != nil {
		return nil, false
	}
	return &DecodedMessageBody{Operation: hipoLogOperations[topic], Value: value}, true
}

func parseHipoLog(topic HipoLogTopic, body []byte, createdAt uint32) (any, error) {
	switch topic {
	case HipoLogLoan:
		return ParseHipoLoanLog(body, createdAt)
	case HipoLogRepayment:
		return ParseHipoRepaymentLog(body, createdAt)
	case HipoLogFinish:
		var log HipoFinishLog
		err := ParseHipoLog(body, &log)
		return log, err
	case HipoLogFailedBurningTokens:
		var log HipoFailedBurningLog
		err := ParseHipoLog(body, &log)
		return log, err
	case HipoLogDeficit:
		var log HipoDeficitLog
		err := ParseHipoLog(body, &log)
		return log, err
	}
	return nil, fmt.Errorf("unknown hipo log topic %d", topic)
}

func hipoCurrentLayout(createdAt uint32) bool {
	// A log of unknown time, 0, is taken as current: a store that does not keep the time of
	// external-out messages would otherwise misread every new log.
	return createdAt == 0 || createdAt >= hipoBorrowerFeeRelease
}

func hipoTopic(dest *ExternalAddress) (HipoLogTopic, bool) {
	if dest == nil || dest.BitsAvailableForRead() != 256 {
		return 0, false
	}
	copied := dest.Copy()
	// ReadUint stops at 64 bits.
	value, err := copied.ReadBigUint(256)
	if err != nil || !value.IsUint64() {
		return 0, false
	}
	switch topic := HipoLogTopic(value.Uint64()); topic {
	case HipoLogLoan, HipoLogRepayment, HipoLogFinish, HipoLogFailedBurningTokens, HipoLogDeficit:
		return topic, true
	}
	return 0, false
}

func decodeHipoBurnerLog(source *ton.AccountID, dest *ExternalAddress, body []byte) (*DecodedMessageBody, bool) {
	if source == nil || *source != references.HipoBurner || dest == nil || dest.BitsAvailableForRead() != 256 {
		return nil, false
	}
	copied := dest.Copy()
	topic, err := copied.ReadBigUint(256)
	if err != nil || !topic.IsUint64() {
		return nil, false
	}
	log, ok := hipoBurnerLogs[topic.Uint64()]
	if !ok {
		return nil, false
	}
	value := log.layout()
	if ParseHipoLog(body, value) != nil {
		return nil, false
	}
	return &DecodedMessageBody{Operation: log.operation, Value: reflect.ValueOf(value).Elem().Interface()}, true
}
