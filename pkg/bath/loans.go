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

type PoolRoundTransitionType string

type (
	LoanRequestAction struct {
		Borrower       tongo.AccountID
		Pool           tongo.AccountID
		RoundSince     uint32
		Amount         *int64
		MinPayment     *int64
		TonAttached    int64
		Implementation core.StakingImplementation
		transfers
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
		transfers
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
		transfers
	}
	PoolRoundTransitionAction struct {
		Pool           tongo.AccountID
		RoundSince     uint32
		Transition     PoolRoundTransitionType
		TotalStaked    *int64
		TotalRecovered *int64
		Implementation core.StakingImplementation
		transfers
	}
)

type BubbleLoanRequest struct {
	LoanRequestAction
	Success bool
}

type BubbleLoanIssue struct {
	LoanIssueAction
	Success bool
}

type BubbleLoanRepayment struct {
	LoanRepaymentAction
	Success bool
}

type BubblePoolRoundTransition struct {
	PoolRoundTransitionAction
	Success bool
}

type (
	// transfers are the coins carried by the messages an action absorbed, so that they count
	// towards the action rather than as an account's unexplained extra
	transfers []transfer
	transfer  struct {
		from   tongo.AccountID
		to     tongo.AccountID
		amount int64
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

func (b BubbleLoanRequest) ToAction() *Action {
	a := b.LoanRequestAction
	return &Action{LoanRequest: &a, Success: b.Success, Type: LoanRequest}
}

func (b BubbleLoanIssue) ToAction() *Action {
	a := b.LoanIssueAction
	return &Action{LoanIssue: &a, Success: b.Success, Type: LoanIssue}
}

func (b BubbleLoanRepayment) ToAction() *Action {
	a := b.LoanRepaymentAction
	return &Action{LoanRepayment: &a, Success: b.Success, Type: LoanRepayment}
}

func (b BubblePoolRoundTransition) ToAction() *Action {
	a := b.PoolRoundTransitionAction
	return &Action{PoolRoundTransition: &a, Success: b.Success, Type: PoolRoundTransition}
}

func (t *transfers) add(bubble *Bubble) {
	tx := bubble.Info.(BubbleTx)
	if tx.inputFrom == nil {
		return
	}
	*t = append(*t, transfer{from: tx.inputFrom.Address, to: tx.account.Address, amount: tx.inputAmount})
}

func (t transfers) contributeToExtra(account tongo.AccountID) int64 {
	var extra int64
	for _, tr := range t {
		extra += detectDirection(account, tr.from, tr.to, tr.amount)
	}
	return extra
}
