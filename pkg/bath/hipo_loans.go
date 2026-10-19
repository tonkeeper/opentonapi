package bath

import (
	"math/big"
	"slices"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
)

// Hipo's loans and rounds. The treasury lends to validators each round and reports the
// amounts only in its logs (see hipo_logs.go), so the loan straws are anchored on a log the
// treasury wrote in its own transaction: anyone can send these ops, but only the treasury can
// write its logs.

const hipoRoundStartedWithin = 65536 / 2

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

// PoolRoundEndedHipoStraw: the stakes are released and the treasury asks for them back.
//
//	finish_participation (external) -> treasury -> recover_stakes -> treasury ->
//	proxy_recover_stake -> each loan
var PoolRoundEndedHipoStraw = Straw[BubblePoolRoundTransition]{
	CheckFuncs: hipoTreasuryExternal(abi.HipoFinanceFinishParticipationExtInMsgOp),
	Builder:    hipoRoundBuilder(PoolRoundEnded),
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
		newAction.add(bubble)
		// When burn_all is sent on recovering the round's last loan, that loan's repayment log
		// names the round. last_bill_burned names it too, but may never come.
		for _, msg := range tx.senderExternalOut {
			if _, value, ok := core.DecodeHipoLog(msg); ok {
				if log, ok := value.(core.HipoRepaymentLog); ok {
					newAction.RoundSince = log.RoundSince
				}
			}
		}
		return nil
	},
	SingleChild: hipoOptionalLastBill(),
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
		newAction.add(bubble)
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
		Builder:    addTransfer[BubbleLoanRequest],
		Optional:   true,
	},
}

// LoanIssueHipoStraw: the treasury lends, and the loan contract stakes it. The elector leg
// below stays an ElectionsDepositStake.
//
//	treasury -> proxy_new_stake -> loan -> new_stake -> elector
var LoanIssueHipoStraw = Straw[BubbleLoanIssue]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceProxyNewStakeMsgOp), func(bubble *Bubble) bool {
		_, _, ok := hipoIssueLog(bubble.Info.(BubbleTx))
		return ok
	}},
	Builder: func(newAction *BubbleLoanIssue, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		log, borrower, _ := hipoIssueLog(tx)
		newAction.Borrower = borrower
		newAction.Pool = references.HipoTreasury
		newAction.Loan = tx.account.Address
		newAction.Implementation = core.StakingImplementationHipo
		newAction.Success = tx.success
		newAction.add(bubble)
		newAction.RoundSince = log.RoundSince
		newAction.Amount = hipoGrams(log.LoanAmount)
		newAction.Accrued = hipoGrams(log.AccrueAmount)
		newAction.Collateral = hipoGrams(log.StakeAmount)
		newAction.MinPayment = hipoGrams(log.MinPayment)
		hipoCredit(bubble, borrower)
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
		_, _, ok := hipoRepaymentLog(hipoLogs(bubble))
		return ok
	}},
	Builder: func(newAction *BubbleLoanRepayment, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		logs := hipoLogs(bubble)
		log, borrower, _ := hipoRepaymentLog(logs)
		newAction.Borrower = borrower
		newAction.Pool = tx.account.Address
		newAction.Implementation = core.StakingImplementationHipo
		newAction.Success = tx.success
		newAction.add(bubble)
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
		hipoCredit(bubble, borrower)
		for _, value := range logs[core.HipoLogDeficit] {
			if deficit, ok := value.(core.HipoDeficitLog); ok {
				shortfall := hipoGrams(deficit.Shortfall)
				newAction.Shortfall = &shortfall
			}
		}
		return nil
	},
	Children: []Straw[BubbleLoanRepayment]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceLoanResultMsgOp), hipoFromTreasury},
			Builder:    addTransfer[BubbleLoanRepayment],
			Optional:   true,
		},
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceTakeBorrowerFeeMsgOp), hipoFromTreasury},
			Builder:    addTransfer[BubbleLoanRepayment],
			Optional:   true,
		},
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceTakeProfitMsgOp), hipoFromTreasury},
			Builder:    addTransfer[BubbleLoanRepayment],
			Optional:   true,
		},
	},
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
		Builder: addTransfer[BubblePoolRoundTransition],
	},
}

// hipoRoundRecoverStraw folds each loan's proxy_recover_stake into the round that asked for
// it; the elector leg below it stays an ElectionsRecoverStake.
var hipoRoundRecoverStraw = Straw[BubblePoolRoundTransition]{
	CheckFuncs: []bubbleCheck{hipoRoundIs(PoolRoundEnded)},
	Builder:    hipoKeepRound,
	SingleChild: &Straw[BubblePoolRoundTransition]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceProxyRecoverStakeMsgOp), hipoFromTreasury},
		Builder:    addTransfer[BubblePoolRoundTransition],
	},
}

// hipoRoundBillStraw folds one burned bill into the settled round, one per pass.
var hipoRoundBillStraw = Straw[BubblePoolRoundTransition]{
	CheckFuncs: []bubbleCheck{hipoRoundIs(PoolRoundSettled)},
	Builder:    hipoKeepRound,
	SingleChild: &Straw[BubblePoolRoundTransition]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.SbtRevokeMsgOp)},
		Builder:    addTransfer[BubblePoolRoundTransition],
		SingleChild: &Straw[BubblePoolRoundTransition]{
			CheckFuncs:  []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceBillBurnedMsgOp)},
			Builder:     addTransfer[BubblePoolRoundTransition],
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
		Builder:    addTransfer[BubblePoolRoundTransition],
		SingleChild: &Straw[BubblePoolRoundTransition]{
			CheckFuncs: []bubbleCheck{IsTx, IsAccount(references.HipoParent), HasOperation(abi.HipoFinanceProxyTokensMintedMsgOp)},
			Builder:    addTransfer[BubblePoolRoundTransition],
		},
	},
}

var hipoLastBillBurned = Straw[BubblePoolRoundTransition]{
	CheckFuncs: []bubbleCheck{IsTx, IsAccount(references.HipoTreasury), HasOperation(abi.HipoFinanceLastBillBurnedMsgOp)},
	Builder: func(newAction *BubblePoolRoundTransition, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.add(bubble)
		if body, ok := tx.decodedBody.Value.(abi.HipoFinanceLastBillBurnedMsgBody); ok {
			newAction.RoundSince = body.RoundSince
		}
		for _, value := range hipoLogs(bubble)[core.HipoLogFinish] {
			if log, ok := value.(core.HipoFinishLog); ok {
				staked, recovered := hipoGrams(log.TotalStaked), hipoGrams(log.TotalRecovered)
				newAction.RoundSince = log.RoundSince
				newAction.TotalStaked = &staked
				newAction.TotalRecovered = &recovered
			}
		}
		return nil
	},
}

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

func hipoRoundIs(transitions ...PoolRoundTransitionType) bubbleCheck {
	return func(bubble *Bubble) bool {
		round, ok := bubble.Info.(BubblePoolRoundTransition)
		if !ok || round.Implementation != core.StakingImplementationHipo {
			return false
		}
		return slices.Contains(transitions, round.Transition)
	}
}

func hipoKeepRound(newAction *BubblePoolRoundTransition, bubble *Bubble) error {
	*newAction = bubble.Info.(BubblePoolRoundTransition)
	// the transfers folded in next must not write into the kept round's backing array
	newAction.transfers = slices.Clip(newAction.transfers)
	return nil
}

func hipoOptionalLastBill() *Straw[BubblePoolRoundTransition] {
	s := Optional(hipoLastBillBurned)
	return &s
}

func hipoIssueLog(tx BubbleTx) (core.HipoLoanLog, tongo.AccountID, bool) {
	// The treasury writes the loan's log::loan straight after sending its proxy_new_stake, in
	// the same transaction. The log names the borrower and the amounts; the message names
	// neither.
	if tx.inputFrom == nil || tx.inputFrom.Address != references.HipoTreasury {
		return core.HipoLoanLog{}, tongo.AccountID{}, false
	}
	for _, msg := range tx.senderExternalOut {
		if msg.CreatedLt != tx.inputLt+1 {
			continue
		}
		topic, value, ok := core.DecodeHipoLog(msg)
		if !ok || topic != core.HipoLogLoan {
			continue
		}
		log, ok := value.(core.HipoLoanLog)
		if !ok {
			return core.HipoLoanLog{}, tongo.AccountID{}, false
		}
		borrower, ok := hipoOwner(log.Borrower)
		return log, borrower, ok
	}
	return core.HipoLoanLog{}, tongo.AccountID{}, false
}

func hipoLogs(bubble *Bubble) map[core.HipoLogTopic][]any {
	// Only the treasury's own transaction is read: the topic is an address, so any contract
	// may emit a message that looks like one of these, and a log is only Hipo's if Hipo's
	// treasury sent it.
	tx, ok := bubble.Info.(BubbleTx)
	if !ok || tx.account.Address != references.HipoTreasury {
		return nil
	}
	var found map[core.HipoLogTopic][]any
	for _, msg := range tx.externalOut {
		topic, value, ok := core.DecodeHipoLog(msg)
		if !ok {
			continue
		}
		if found == nil {
			found = make(map[core.HipoLogTopic][]any, 2)
		}
		found[topic] = append(found[topic], value)
	}
	return found
}

func hipoRepaymentLog(logs map[core.HipoLogTopic][]any) (core.HipoRepaymentLog, tongo.AccountID, bool) {
	repayments := logs[core.HipoLogRepayment]
	if len(repayments) != 1 {
		return core.HipoRepaymentLog{}, tongo.AccountID{}, false
	}
	log, ok := repayments[0].(core.HipoRepaymentLog)
	if !ok {
		return core.HipoRepaymentLog{}, tongo.AccountID{}, false
	}
	borrower, ok := hipoOwner(log.Borrower)
	return log, borrower, ok
}

func hipoGrams(g tlb.Grams) int64 {
	return int64(g)
}

func hipoVarCoins(c tlb.VarUInteger16) int64 {
	// A request may claim any 120-bit number; one past int64 is reported as 0 rather than
	// wrapped, and the treasury refuses it anyway.
	v := big.Int(c)
	if !v.IsInt64() {
		return 0
	}
	return v.Int64()
}

func addTransfer[T any, P interface {
	*T
	add(*Bubble)
}](newAction P, bubble *Bubble) error {
	newAction.add(bubble)
	return nil
}
