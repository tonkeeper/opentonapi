package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
)

func (h *Handler) GetAccountInscriptions(ctx context.Context, params oas.GetAccountInscriptionsParams) (*oas.InscriptionBalances, error) {
	a, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	balances, err := h.storage.GetInscriptionBalancesByAccount(ctx, a.ID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var inscriptions []oas.InscriptionBalance
	for _, b := range balances {
		inscriptions = append(inscriptions, oas.InscriptionBalance{
			Type:     oas.InscriptionBalanceTypeTon20,
			Ticker:   b.Ticker,
			Balance:  b.Amount.String(),
			Decimals: 9,
		})
	}
	return &oas.InscriptionBalances{
		Inscriptions: inscriptions,
	}, nil
}

func (h *Handler) GetInscriptionOpTemplate(ctx context.Context, params oas.GetInscriptionOpTemplateParams) (*oas.GetInscriptionOpTemplateOK, error) {
	if params.Type != oas.GetInscriptionOpTemplateTypeTon20 ||
		params.Operation != oas.GetInscriptionOpTemplateOperationTransfer ||
		!params.Destination.Set {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("invalid request"))
	}
	payload := struct {
		Protocol  string `json:"p"`
		Operation string `json:"op"`
		Ticker    string `json:"tick"`
		To        string `json:"to"`
		Amount    string `json:"amt"`
		Memo      string `json:"memo,omitempty"`
	}{
		Protocol:  "ton-20",
		Operation: "transfer",
		Ticker:    params.Ticker,
		To:        params.Destination.Value,
		Amount:    params.Amount,
	}
	if params.Comment.Set {
		payload.Memo = params.Comment.Value
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}

	return &oas.GetInscriptionOpTemplateOK{
		Comment:     "data:application/json," + string(b),
		Destination: params.Who,
	}, nil
}
