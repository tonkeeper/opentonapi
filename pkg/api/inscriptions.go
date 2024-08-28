package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/arnac-io/opentonapi/pkg/bath"
	"github.com/arnac-io/opentonapi/pkg/core"
	"github.com/arnac-io/opentonapi/pkg/oas"
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

func (h *Handler) GetAccountInscriptionsHistory(ctx context.Context, params oas.GetAccountInscriptionsHistoryParams) (*oas.AccountEvents, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	beforeLT := int64(1 << 62)
	if params.BeforeLt.Set {
		beforeLT = params.BeforeLt.Value
	}
	msgs, err := h.storage.GetInscriptionsHistoryByAccount(ctx, account.ID, nil, beforeLT, params.Limit.Value)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.AccountEvents{}, nil
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	events := bath.ConvertToInscriptionActions(msgs)
	var resp oas.AccountEvents
	for hash, actions := range events {
		event := oas.AccountEvent{
			EventID: hash.Hex(),
			Account: convertAccountAddress(account.ID, h.addressBook),
		}
		for _, a := range actions {
			action, err := h.convertAction(ctx, &account.ID, a, params.AcceptLanguage)
			if err != nil {
				return nil, toError(http.StatusInternalServerError, err)
			}
			event.Actions = append(event.Actions, action)
		}
		resp.Events = append(resp.Events, event)
	}
	return &resp, nil
}

func (h *Handler) GetAccountInscriptionsHistoryByTicker(ctx context.Context, params oas.GetAccountInscriptionsHistoryByTickerParams) (*oas.AccountEvents, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	beforeLT := int64(1 << 62)
	if params.BeforeLt.Set {
		beforeLT = params.BeforeLt.Value
	}
	msgs, err := h.storage.GetInscriptionsHistoryByAccount(ctx, account.ID, &params.Ticker, beforeLT, params.Limit.Value)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.AccountEvents{}, nil
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	events := bath.ConvertToInscriptionActions(msgs)
	var resp oas.AccountEvents
	for hash, actions := range events {
		event := oas.AccountEvent{
			EventID: hash.Hex(),
			Account: convertAccountAddress(account.ID, h.addressBook),
		}
		for _, a := range actions {
			action, err := h.convertAction(ctx, &account.ID, a, params.AcceptLanguage)
			if err != nil {
				return nil, toError(http.StatusInternalServerError, err)
			}
			event.Actions = append(event.Actions, action)
		}
		resp.Events = append(resp.Events, event)
	}
	return &resp, nil
}
