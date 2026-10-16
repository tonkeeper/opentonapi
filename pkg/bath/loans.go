package bath

import (
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
)

// A lending pool lends its stakers' coins to validators, one validation round at a time. These
// actions show that side of a pool: a validator asks for a loan, the pool lends and the loan is
// staked, the elector returns it with the round's reward and the pool splits that reward, and the
// round moves through its stages. They are generic, like DepositStake, and say which pool
// implementation produced them.

type PoolRoundTransitionType string

const (
	// PoolRoundElection: the pool decided which loans to make and staked them with the elector.
	PoolRoundElection PoolRoundTransitionType = "election"
	// PoolRoundBegan: the validation round started.
	PoolRoundBegan PoolRoundTransitionType = "began"
	// PoolRoundValidated: the round's validators finished validating, and the stakes are held
	// for a while before the elector releases them.
	PoolRoundValidated PoolRoundTransitionType = "validated"
	// PoolRoundEnded: the round's stakes were released and the pool is recovering them.
	PoolRoundEnded PoolRoundTransitionType = "ended"
	// PoolRoundSettled: every loan was recovered and the round's deposits and withdrawals
	// were settled.
	PoolRoundSettled PoolRoundTransitionType = "settled"
)

type (
	LoanRequestAction struct {
		Borrower       tongo.AccountID
		Pool           tongo.AccountID
		RoundSince     uint32
		Amount         int64
		MinPayment     int64
		TonAttached    int64
		Implementation core.StakingImplementation
	}
	LoanIssueAction struct {
		Borrower       tongo.AccountID
		Pool           tongo.AccountID
		Loan           tongo.AccountID
		RoundSince     uint32
		Amount         int64
		Accrued        int64
		Collateral     int64
		MinPayment     int64
		Implementation core.StakingImplementation
		// what the pool sent the loan contract: the stake and the fees to place it
		tonAttached int64
	}
	LoanRepaymentAction struct {
		Borrower       tongo.AccountID
		Pool           tongo.AccountID
		Loan           tongo.AccountID
		RoundSince     uint32
		Amount         int64
		Accrued        int64
		Recovered      int64
		PoolReward     int64
		ProtocolFee    int64
		BorrowerPayout int64
		BurnAmount     int64
		Shortfall      *int64
		Implementation core.StakingImplementation
		// what reached the borrower, which is the payout less the cost of sending it
		payoutReceived int64
	}
	PoolRoundTransitionAction struct {
		Pool           tongo.AccountID
		RoundSince     uint32
		Transition     PoolRoundTransitionType
		TotalStaked    *int64
		TotalRecovered *int64
		Implementation core.StakingImplementation
	}
)

func (a *LoanRequestAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Borrower, a.Pool}
}

func (a *LoanIssueAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Borrower, a.Pool, a.Loan}
}

func (a *LoanRepaymentAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Borrower, a.Pool, a.Loan}
}

func (a *PoolRoundTransitionAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Pool}
}

type BubbleLoanRequest struct {
	LoanRequestAction
	Success bool
}

func (b BubbleLoanRequest) ToAction() *Action {
	a := b.LoanRequestAction
	return &Action{LoanRequest: &a, Success: b.Success, Type: LoanRequest}
}

type BubbleLoanIssue struct {
	LoanIssueAction
	Success bool
}

func (b BubbleLoanIssue) ToAction() *Action {
	a := b.LoanIssueAction
	return &Action{LoanIssue: &a, Success: b.Success, Type: LoanIssue}
}

type BubbleLoanRepayment struct {
	LoanRepaymentAction
	Success bool
}

func (b BubbleLoanRepayment) ToAction() *Action {
	a := b.LoanRepaymentAction
	return &Action{LoanRepayment: &a, Success: b.Success, Type: LoanRepayment}
}

type BubblePoolRoundTransition struct {
	PoolRoundTransitionAction
	Success bool
}

func (b BubblePoolRoundTransition) ToAction() *Action {
	a := b.PoolRoundTransitionAction
	return &Action{PoolRoundTransition: &a, Success: b.Success, Type: PoolRoundTransition}
}
