package bath

import (
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
)

// Hipo's loans and rounds. The treasury lends to validators each round and reports the
// amounts only in its logs (see hipo_logs.go), so the loan straws are anchored on a log the
// treasury wrote in its own transaction: anyone can send these ops, but only the treasury can
// write its logs.

// hipoLogs collects the logs of one treasury transaction, by topic. Only the treasury's own
// transaction is read: the topic is an address, so any contract may emit a message that
// looks like one of these, and a log is only Hipo's if Hipo's treasury sent it.
func hipoLogs(bubble *Bubble) map[core.HipoLogTopic][]core.Message {
	tx, ok := bubble.Info.(BubbleTx)
	if !ok || tx.account.Address != references.HipoTreasury {
		return nil
	}
	var found map[core.HipoLogTopic][]core.Message
	for _, msg := range tx.externalOut {
		topic, ok := core.HipoLogTopicOf(msg)
		if !ok {
			continue
		}
		if found == nil {
			found = make(map[core.HipoLogTopic][]core.Message, 2)
		}
		found[topic] = append(found[topic], msg)
	}
	return found
}

func hipoGrams(g tlb.Grams) int64 {
	return int64(g)
}

// hipoVarCoins reads an amount from a request, which may claim any 120-bit number; one past
// int64 is reported as 0 rather than wrapped, and the treasury refuses it anyway.
func hipoVarCoins(c tlb.VarUInteger16) int64 {
	v := big.Int(c)
	if !v.IsInt64() {
		return 0
	}
	return v.Int64()
}

// hipoTreasuryExternal matches one of the three ops the treasury takes as an external message.
func hipoTreasuryExternal(op abi.MsgOpName) []bubbleCheck {
	return []bubbleCheck{IsTx, IsAccount(references.HipoTreasury), HasOperation(op), func(bubble *Bubble) bool {
		return bubble.Info.(BubbleTx).external
	}}
}

func hipoRoundBuilder(transition PoolRoundTransitionType) func(*BubblePoolRoundTransition, *Bubble) error {
	return func(newAction *BubblePoolRoundTransition, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Pool = tx.account.Address
		newAction.Transition = transition
		newAction.Implementation = core.StakingImplementationHipo
		newAction.Success = tx.success
		switch body := tx.decodedBody.Value.(type) {
		case abi.HipoFinanceParticipateInElectionExtInMsgBody:
			newAction.RoundSince = body.RoundSince
		case abi.HipoFinanceVsetChangedExtInMsgBody:
			newAction.RoundSince = body.RoundSince
		case abi.HipoFinanceFinishParticipationExtInMsgBody:
			newAction.RoundSince = body.RoundSince
		}
		return nil
	}
}

// PoolRoundElectionHipoStraw: bidding has closed and the treasury decides the round's loans.
//
//	participate_in_election (external) -> treasury -> decide_loan_requests -> treasury ->
//	process_loan_requests -> treasury
var PoolRoundElectionHipoStraw = Straw[BubblePoolRoundTransition]{
	CheckFuncs: hipoTreasuryExternal(abi.HipoFinanceParticipateInElectionExtInMsgOp),
	Builder:    hipoRoundBuilder(PoolRoundElection),
}

// PoolRoundBeganHipoStraw: the validator set changed. The treasury is told once for each
// round it takes part in: the round that has just begun, and the one before it, whose
// validators have just stopped. Which one this is shows in how long ago the round started.
var PoolRoundBeganHipoStraw = Straw[BubblePoolRoundTransition]{
	CheckFuncs: hipoTreasuryExternal(abi.HipoFinanceVsetChangedExtInMsgOp),
	Builder: func(newAction *BubblePoolRoundTransition, bubble *Bubble) error {
		if err := hipoRoundBuilder(PoolRoundBegan)(newAction, bubble); err != nil {
			return err
		}
		// The change comes moments after the new round starts and a whole round after the
		// old one did, so half a round tells them apart even if the message is hours late.
		if bubble.Info.(BubbleTx).utime-int64(newAction.RoundSince) > hipoRoundStartedWithin {
			newAction.Transition = PoolRoundValidated
		}
		return nil
	},
}

const hipoRoundStartedWithin = 65536 / 2

// PoolRoundEndedHipoStraw: the stakes are released and the treasury asks for them back.
//
//	finish_participation (external) -> treasury -> recover_stakes -> treasury ->
//	proxy_recover_stake -> each loan
var PoolRoundEndedHipoStraw = Straw[BubblePoolRoundTransition]{
	CheckFuncs: hipoTreasuryExternal(abi.HipoFinanceFinishParticipationExtInMsgOp),
	Builder:    hipoRoundBuilder(PoolRoundEnded),
}

func hipoRoundIs(transitions ...PoolRoundTransitionType) bubbleCheck {
	return func(bubble *Bubble) bool {
		round, ok := bubble.Info.(BubblePoolRoundTransition)
		if !ok || round.Implementation != core.StakingImplementationHipo {
			return false
		}
		for _, t := range transitions {
			if round.Transition == t {
				return true
			}
		}
		return false
	}
}

func hipoKeepRound(newAction *BubblePoolRoundTransition, bubble *Bubble) error {
	*newAction = bubble.Info.(BubblePoolRoundTransition)
	return nil
}

// hipoRoundStepStraw folds the treasury's messages to itself into the round, one per pass:
// there are as many as the round's loans need, since each one stops at a gas budget and sends
// the next.
var hipoRoundStepStraw = Straw[BubblePoolRoundTransition]{
	CheckFuncs: []bubbleCheck{hipoRoundIs(PoolRoundElection, PoolRoundEnded)},
	Builder:    hipoKeepRound,
	SingleChild: &Straw[BubblePoolRoundTransition]{
		CheckFuncs: []bubbleCheck{IsTx, IsAccount(references.HipoTreasury), hipoFromTreasury, Or(
			Or(HasOperation(abi.HipoFinanceDecideLoanRequestsMsgOp), HasOperation(abi.HipoFinanceProcessLoanRequestsMsgOp)),
			HasOperation(abi.HipoFinanceRecoverStakesMsgOp),
		)},
	},
}

// hipoRoundRecoverStraw folds each loan's proxy_recover_stake into the round that asked for
// it; the elector leg below it stays an ElectionsRecoverStake.
var hipoRoundRecoverStraw = Straw[BubblePoolRoundTransition]{
	CheckFuncs: []bubbleCheck{hipoRoundIs(PoolRoundEnded)},
	Builder:    hipoKeepRound,
	SingleChild: &Straw[BubblePoolRoundTransition]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceProxyRecoverStakeMsgOp), hipoFromTreasury},
	},
}

// PoolRoundSettledHipoStraw: every loan is back, and the round's bills are burned. The bills'
// own settlements below stay DepositStake, JettonMint and WithdrawStake actions; the round
// folds in the burning machinery and, from log::finish, the round's totals.
//
//	treasury -> burn_all -> collection -> revoke -> bill -> bill_burned -> collection ->
//	... -> last_bill_burned -> treasury
var PoolRoundSettledHipoStraw = Straw[BubblePoolRoundTransition]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceBurnAllMsgOp), hipoFromTreasury},
	Builder: func(newAction *BubblePoolRoundTransition, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Pool = references.HipoTreasury
		newAction.Transition = PoolRoundSettled
		newAction.Implementation = core.StakingImplementationHipo
		newAction.Success = tx.success
		return nil
	},
	SingleChild: hipoOptionalLastBill(),
}

// hipoRoundBillStraw folds one burned bill into the settled round, one per pass.
var hipoRoundBillStraw = Straw[BubblePoolRoundTransition]{
	CheckFuncs: []bubbleCheck{hipoRoundIs(PoolRoundSettled)},
	Builder:    hipoKeepRound,
	SingleChild: &Straw[BubblePoolRoundTransition]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.SbtRevokeMsgOp)},
		SingleChild: &Straw[BubblePoolRoundTransition]{
			CheckFuncs:  []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceBillBurnedMsgOp)},
			SingleChild: hipoOptionalLastBill(),
		},
	},
}

// hipoRoundMintStraw folds the treasury's half of a deposit bill's settlement into the settled
// round, one per pass. The hGRAM it mints stays a JettonMint, from JettonMintHipoStraw.
//
//	bill_burned -> collection -> mint_tokens -> treasury -> proxy_tokens_minted -> parent ->
//	tokens_minted -> hGRAM wallet
var hipoRoundMintStraw = Straw[BubblePoolRoundTransition]{
	CheckFuncs: []bubbleCheck{hipoRoundIs(PoolRoundSettled)},
	Builder:    hipoKeepRound,
	SingleChild: &Straw[BubblePoolRoundTransition]{
		CheckFuncs: []bubbleCheck{IsTx, IsAccount(references.HipoTreasury), HasOperation(abi.HipoFinanceMintTokensMsgOp)},
		SingleChild: &Straw[BubblePoolRoundTransition]{
			CheckFuncs: []bubbleCheck{IsTx, IsAccount(references.HipoParent), HasOperation(abi.HipoFinanceProxyTokensMintedMsgOp)},
		},
	},
}

func hipoOptionalLastBill() *Straw[BubblePoolRoundTransition] {
	s := Optional(hipoLastBillBurned)
	return &s
}

var hipoLastBillBurned = Straw[BubblePoolRoundTransition]{
	CheckFuncs: []bubbleCheck{IsTx, IsAccount(references.HipoTreasury), HasOperation(abi.HipoFinanceLastBillBurnedMsgOp)},
	Builder: func(newAction *BubblePoolRoundTransition, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		if body, ok := tx.decodedBody.Value.(abi.HipoFinanceLastBillBurnedMsgBody); ok {
			newAction.RoundSince = body.RoundSince
		}
		for _, raw := range hipoLogs(bubble)[core.HipoLogFinish] {
			var log core.HipoFinishLog
			if core.ParseHipoLog(raw.Body, &log) != nil {
				continue
			}
			staked, recovered := hipoGrams(log.TotalStaked), hipoGrams(log.TotalRecovered)
			newAction.RoundSince = log.RoundSince
			newAction.TotalStaked = &staked
			newAction.TotalRecovered = &recovered
		}
		return nil
	},
}

// LoanRequestHipoStraw: a validator asks for a loan. The treasury accepts any of the request
// layouts it has used, and keeps the coins sent with it as collateral.
var LoanRequestHipoStraw = Straw[BubbleLoanRequest]{
	CheckFuncs: []bubbleCheck{IsTx, IsAccount(references.HipoTreasury), Or(
		Or(HasOperation(abi.HipoFinanceRequestLoanMsgOp), HasOperation(abi.HipoFinanceRequestLoanV2MsgOp)),
		Or(HasOperation(abi.HipoFinanceRequestLoanV3MsgOp), HasOperation(abi.HipoFinanceRequestLoanV4MsgOp)),
	)},
	Builder: func(newAction *BubbleLoanRequest, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Pool = tx.account.Address
		newAction.Implementation = core.StakingImplementationHipo
		newAction.TonAttached = tx.inputAmount
		newAction.Success = tx.success
		if tx.inputFrom != nil {
			newAction.Borrower = tx.inputFrom.Address
		}
		switch body := tx.decodedBody.Value.(type) {
		case abi.HipoFinanceRequestLoanMsgBody:
			newAction.RoundSince, newAction.Amount, newAction.MinPayment = body.RoundSince, hipoVarCoins(body.LoanAmount), hipoVarCoins(body.MinPayment)
		case abi.HipoFinanceRequestLoanV2MsgBody:
			newAction.RoundSince, newAction.Amount, newAction.MinPayment = body.RoundSince, hipoVarCoins(body.LoanAmount), hipoVarCoins(body.MinPayment)
		case abi.HipoFinanceRequestLoanV3MsgBody:
			newAction.RoundSince, newAction.Amount, newAction.MinPayment = body.RoundSince, hipoVarCoins(body.LoanAmount), hipoVarCoins(body.MinPayment)
		case abi.HipoFinanceRequestLoanV4MsgBody:
			newAction.RoundSince, newAction.Amount, newAction.MinPayment = body.RoundSince, hipoVarCoins(body.LoanAmount), hipoVarCoins(body.MinPayment)
		}
		return nil
	},
	SingleChild: &Straw[BubbleLoanRequest]{
		CheckFuncs: []bubbleCheck{IsTx, IsBounced},
		Optional:   true,
	},
}

// hipoIssueLog finds the log::loan the treasury wrote straight after sending this loan's
// proxy_new_stake, in the same transaction. The log names the borrower and the amounts; the
// message names neither.
func hipoIssueLog(tx BubbleTx) (core.HipoLoanLog, bool) {
	if tx.inputFrom == nil || tx.inputFrom.Address != references.HipoTreasury {
		return core.HipoLoanLog{}, false
	}
	for _, msg := range tx.senderExternalOut {
		if msg.CreatedLt != tx.inputLt+1 {
			continue
		}
		if topic, ok := core.HipoLogTopicOf(msg); !ok || topic != core.HipoLogLoan {
			continue
		}
		log, err := core.ParseHipoLoanLog(msg.Body, msg.CreatedAt)
		if err != nil {
			return core.HipoLoanLog{}, false
		}
		return log, true
	}
	return core.HipoLoanLog{}, false
}

// LoanIssueHipoStraw: the treasury lends, and the loan contract stakes it. The elector leg
// below stays an ElectionsDepositStake.
//
//	treasury -> proxy_new_stake -> loan -> new_stake -> elector
var LoanIssueHipoStraw = Straw[BubbleLoanIssue]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceProxyNewStakeMsgOp), func(bubble *Bubble) bool {
		_, ok := hipoIssueLog(bubble.Info.(BubbleTx))
		return ok
	}},
	Builder: func(newAction *BubbleLoanIssue, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		log, _ := hipoIssueLog(tx)
		newAction.Pool = references.HipoTreasury
		newAction.Loan = tx.account.Address
		newAction.Implementation = core.StakingImplementationHipo
		newAction.Success = tx.success
		newAction.tonAttached = tx.inputAmount
		newAction.RoundSince = log.RoundSince
		newAction.Amount = hipoGrams(log.LoanAmount)
		newAction.Accrued = hipoGrams(log.AccrueAmount)
		newAction.Collateral = hipoGrams(log.StakeAmount)
		newAction.MinPayment = hipoGrams(log.MinPayment)
		if borrower, ok := hipoOwner(log.Borrower); ok {
			newAction.Borrower = borrower
			hipoCredit(bubble, borrower)
		}
		return nil
	},
	SingleChild: &Straw[BubbleLoanIssue]{
		CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
		Optional:   true,
	},
}

// LoanRepaymentHipoStraw: a loan is back from the elector and the treasury splits it, by
// log::repayment, between the stakers, the governor, the burner and the borrower.
//
//	loan -> recover_stake_result -> treasury -> loan_result -> borrower
//	                                         -> take_borrower_fee -> burner
//	                                         -> take_profit -> governor
var LoanRepaymentHipoStraw = Straw[BubbleLoanRepayment]{
	CheckFuncs: []bubbleCheck{IsTx, IsAccount(references.HipoTreasury), HasOperation(abi.HipoFinanceRecoverStakeResultMsgOp), func(bubble *Bubble) bool {
		return len(hipoLogs(bubble)[core.HipoLogRepayment]) == 1
	}},
	Builder: func(newAction *BubbleLoanRepayment, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		logs := hipoLogs(bubble)
		repayment := logs[core.HipoLogRepayment][0]
		log, err := core.ParseHipoRepaymentLog(repayment.Body, repayment.CreatedAt)
		if err != nil {
			return err
		}
		newAction.Pool = tx.account.Address
		newAction.Implementation = core.StakingImplementationHipo
		newAction.Success = tx.success
		if tx.inputFrom != nil {
			newAction.Loan = tx.inputFrom.Address
		}
		newAction.RoundSince = log.RoundSince
		newAction.Amount = hipoGrams(log.LoanAmount)
		newAction.Accrued = hipoGrams(log.AccrueAmount)
		newAction.Recovered = hipoGrams(log.RepaymentAmount)
		newAction.PoolReward = hipoGrams(log.StakersShare)
		newAction.ProtocolFee = hipoGrams(log.GovernorShare)
		newAction.BorrowerPayout = hipoGrams(log.BorrowerShare)
		newAction.BurnAmount = hipoGrams(log.BurnShare)
		if borrower, ok := hipoOwner(log.Borrower); ok {
			newAction.Borrower = borrower
			hipoCredit(bubble, borrower)
		}
		for _, raw := range logs[core.HipoLogDeficit] {
			var deficit core.HipoDeficitLog
			if core.ParseHipoLog(raw.Body, &deficit) == nil {
				shortfall := hipoGrams(deficit.Shortfall)
				newAction.Shortfall = &shortfall
			}
		}
		return nil
	},
	Children: []Straw[BubbleLoanRepayment]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceLoanResultMsgOp), hipoFromTreasury},
			Builder: func(newAction *BubbleLoanRepayment, bubble *Bubble) error {
				newAction.payoutReceived = bubble.Info.(BubbleTx).inputAmount
				return nil
			},
			Optional: true,
		},
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceTakeBorrowerFeeMsgOp), hipoFromTreasury},
			Optional:   true,
		},
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceTakeProfitMsgOp), hipoFromTreasury},
			Optional:   true,
		},
	},
}
