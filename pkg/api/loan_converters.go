package api

import (
	"time"

	"github.com/tonkeeper/opentonapi/pkg/api/i18n"
	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
)

// roundStart names a validation round by when it starts, which is what round_since is.
func roundStart(roundSince uint32) string {
	return time.Unix(int64(roundSince), 0).UTC().Format("2006-01-02 15:04 UTC")
}

func (h *Handler) convertLoanRequestAction(a *bath.LoanRequestAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptLoanRequestAction, oas.ActionSimplePreview) {
	var action oas.OptLoanRequestAction
	action.SetTo(oas.LoanRequestAction{
		Borrower:       convertAccountAddress(a.Borrower, h.addressBook),
		Pool:           convertAccountAddress(a.Pool, h.addressBook),
		RoundSince:     int64(a.RoundSince),
		Amount:         a.Amount,
		MinPayment:     a.MinPayment,
		TonAttached:    a.TonAttached,
		Implementation: oas.PoolImplementationType(a.Implementation),
	})
	value := i18n.FormatGrams(a.Amount)
	return action, oas.ActionSimplePreview{
		Name: "Loan Request",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "loanRequestAction",
				Other: "Requesting a loan of {{.Amount}} for the validation round starting {{.Round}}",
			},
			TemplateData: i18n.Template{"Amount": value, "Round": roundStart(a.RoundSince)},
		}),
		Value:    oas.NewOptString(value),
		Accounts: distinctAccounts(viewer, h.addressBook, &a.Borrower, &a.Pool),
	}
}

func (h *Handler) convertLoanIssueAction(a *bath.LoanIssueAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptLoanIssueAction, oas.ActionSimplePreview) {
	var action oas.OptLoanIssueAction
	action.SetTo(oas.LoanIssueAction{
		Borrower:       convertAccountAddress(a.Borrower, h.addressBook),
		Pool:           convertAccountAddress(a.Pool, h.addressBook),
		Loan:           convertAccountAddress(a.Loan, h.addressBook),
		RoundSince:     int64(a.RoundSince),
		Amount:         a.Amount,
		Accrued:        a.Accrued,
		Collateral:     a.Collateral,
		MinPayment:     a.MinPayment,
		Implementation: oas.PoolImplementationType(a.Implementation),
	})
	value := i18n.FormatGrams(a.Amount + a.Accrued)
	return action, oas.ActionSimplePreview{
		Name: "Loan Issue",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "loanIssueAction",
				Other: "Lending {{.Amount}} to a validator for the round starting {{.Round}}",
			},
			TemplateData: i18n.Template{"Amount": value, "Round": roundStart(a.RoundSince)},
		}),
		Value:    oas.NewOptString(value),
		Accounts: distinctAccounts(viewer, h.addressBook, &a.Pool, &a.Borrower, &a.Loan),
	}
}

func (h *Handler) convertLoanRepaymentAction(a *bath.LoanRepaymentAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptLoanRepaymentAction, oas.ActionSimplePreview) {
	repayment := oas.LoanRepaymentAction{
		Borrower:       convertAccountAddress(a.Borrower, h.addressBook),
		Pool:           convertAccountAddress(a.Pool, h.addressBook),
		Loan:           convertAccountAddress(a.Loan, h.addressBook),
		RoundSince:     int64(a.RoundSince),
		Amount:         a.Amount,
		Accrued:        a.Accrued,
		Recovered:      a.Recovered,
		PoolReward:     a.PoolReward,
		ProtocolFee:    a.ProtocolFee,
		BorrowerPayout: a.BorrowerPayout,
		BurnAmount:     a.BurnAmount,
		Implementation: oas.PoolImplementationType(a.Implementation),
	}
	if a.Shortfall != nil {
		repayment.Shortfall = oas.NewOptInt64(*a.Shortfall)
	}
	var action oas.OptLoanRepaymentAction
	action.SetTo(repayment)
	loan := i18n.FormatGrams(a.Amount + a.Accrued)
	reward := i18n.FormatGrams(a.PoolReward)
	return action, oas.ActionSimplePreview{
		Name: "Loan Repayment",
		Description: i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &i18n.M{
				ID:    "loanRepaymentAction",
				Other: "Loan of {{.Amount}} repaid, stakers earned {{.Reward}}",
			},
			TemplateData: i18n.Template{"Amount": loan, "Reward": reward},
		}),
		Value:    oas.NewOptString(reward),
		Accounts: distinctAccounts(viewer, h.addressBook, &a.Pool, &a.Borrower, &a.Loan),
	}
}

func (h *Handler) convertPoolRoundTransitionAction(a *bath.PoolRoundTransitionAction, acceptLanguage string, viewer *tongo.AccountID) (oas.OptPoolRoundTransitionAction, oas.ActionSimplePreview) {
	transition := oas.PoolRoundTransitionAction{
		Pool:           convertAccountAddress(a.Pool, h.addressBook),
		RoundSince:     int64(a.RoundSince),
		Transition:     oas.PoolRoundTransition(a.Transition),
		Implementation: oas.PoolImplementationType(a.Implementation),
	}
	if a.TotalStaked != nil {
		transition.TotalStaked = oas.NewOptInt64(*a.TotalStaked)
	}
	if a.TotalRecovered != nil {
		transition.TotalRecovered = oas.NewOptInt64(*a.TotalRecovered)
	}
	var action oas.OptPoolRoundTransitionAction
	action.SetTo(transition)
	messages := map[bath.PoolRoundTransitionType]i18n.M{
		bath.PoolRoundElection:  {ID: "poolRoundElection", Other: "Loans decided for the validation round starting {{.Round}}"},
		bath.PoolRoundBegan:     {ID: "poolRoundBegan", Other: "Validation round starting {{.Round}} began"},
		bath.PoolRoundValidated: {ID: "poolRoundValidated", Other: "Validation round starting {{.Round}} finished, its stakes are held"},
		bath.PoolRoundEnded:     {ID: "poolRoundEnded", Other: "Stakes of the validation round starting {{.Round}} released"},
		bath.PoolRoundSettled:   {ID: "poolRoundSettled", Other: "Validation round starting {{.Round}} settled"},
	}
	preview := oas.ActionSimplePreview{
		Name:     "Validation Round",
		Accounts: distinctAccounts(viewer, h.addressBook, &a.Pool),
	}
	if message, ok := messages[a.Transition]; ok {
		preview.Description = i18n.T(acceptLanguage, i18n.C{
			DefaultMessage: &message,
			TemplateData:   i18n.Template{"Round": roundStart(a.RoundSince)},
		})
	}
	if a.TotalStaked != nil && a.TotalRecovered != nil {
		preview.Value = oas.NewOptString(i18n.FormatGrams(*a.TotalRecovered - *a.TotalStaked))
	}
	return action, preview
}
