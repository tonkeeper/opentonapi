package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
)

func (h *Handler) GetAccountJettonsBalances(ctx context.Context, params oas.GetAccountJettonsBalancesParams) (*oas.JettonsBalances, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	wallets, err := h.storage.GetJettonWalletsByOwnerAddress(ctx, account.ID, nil, true, slices.Contains(params.SupportedExtensions, "custom_payload"))
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.JettonsBalances{}, nil
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var balances = oas.JettonsBalances{
		Balances: make([]oas.JettonBalance, 0, len(wallets)),
	}
	for _, wallet := range wallets {
		jettonBalance, err := h.convertJettonBalance(ctx, wallet, params.Currencies, nil)
		if err != nil {
			continue
		}
		balances.Balances = append(balances.Balances, jettonBalance)
	}
	return &balances, nil
}

func (h *Handler) GetAccountJettonBalance(ctx context.Context, params oas.GetAccountJettonBalanceParams) (*oas.JettonBalance, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	jettonAccount, err := tongo.ParseAddress(params.JettonID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	wallets, err := h.storage.GetJettonWalletsByOwnerAddress(ctx, account.ID, &jettonAccount.ID, true, slices.Contains(params.SupportedExtensions, "custom_payload"))
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	if len(wallets) == 0 {
		return nil, toError(http.StatusNotFound, fmt.Errorf("account %v has no jetton wallet %v", account.ID, jettonAccount.ID))
	}
	jettonBalance, err := h.convertJettonBalance(ctx, wallets[0], params.Currencies, nil)
	if err != nil {
		return nil, err
	}
	return &jettonBalance, nil
}

func (h *Handler) GetJettonInfo(ctx context.Context, params oas.GetJettonInfoParams) (*oas.JettonInfo, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	master, err := h.storage.GetJettonMasterData(ctx, account.ID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	holders, err := h.storage.GetJettonsHoldersCount(ctx, []tongo.AccountID{account.ID})
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	scaledUiParams, err := h.storage.GetScaledUIParameters(ctx, master.Address, nil)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	converted := h.convertJettonInfo(ctx, master, holders, scaledUiParams)
	return &converted, nil
}

func (h *Handler) GetAccountJettonsHistory(ctx context.Context, params oas.GetAccountJettonsHistoryParams) (*oas.JettonOperations, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	history, err := h.storage.GetAccountJettonsHistory(ctx, account.ID, params.Limit, optIntToPointer(params.BeforeLt), nil, nil)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.JettonOperations{}, nil
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var res oas.JettonOperations
	for _, op := range history {
		convertedOp, err := h.convertJettonOperation(ctx, op)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		res.Operations = append(res.Operations, convertedOp)
		if len(history) == params.Limit {
			res.NextFrom = oas.NewOptInt64(int64(op.Lt))
		}
	}
	return &res, nil
}

func (h *Handler) GetAccountJettonHistoryByID(ctx context.Context, params oas.GetAccountJettonHistoryByIDParams) (*oas.AccountEvents, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	jettonMasterAccount, err := tongo.ParseAddress(params.JettonID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	traceIDs, err := h.storage.GetAccountJettonHistoryByID(ctx, account.ID, jettonMasterAccount.ID, params.Limit, optIntToPointer(params.BeforeLt), optIntToPointer(params.StartDate), optIntToPointer(params.EndDate))
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.AccountEvents{}, nil
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var eventIDs []string
	for _, traceID := range traceIDs {
		eventIDs = append(eventIDs, traceID.Hex())
	}
	events, lastLT, err := h.convertJettonHistory(ctx, account.ID, &jettonMasterAccount.ID, traceIDs, params.AcceptLanguage)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &oas.AccountEvents{Events: events, NextFrom: lastLT}, nil
}

func (h *Handler) GetJettonAccountHistoryByID(ctx context.Context, params oas.GetJettonAccountHistoryByIDParams) (*oas.JettonOperations, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	jettonMasterAccount, err := tongo.ParseAddress(params.JettonID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	history, err := h.storage.GetJettonAccountHistoryByID(ctx, account.ID, jettonMasterAccount.ID, params.Limit, optIntToPointer(params.BeforeLt), optIntToPointer(params.StartDate), optIntToPointer(params.EndDate))
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.JettonOperations{}, nil
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	res := oas.JettonOperations{}
	for _, op := range history {
		convertedOp, err := h.convertJettonOperation(ctx, op)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		res.Operations = append(res.Operations, convertedOp)
		if len(history) == params.Limit {
			res.NextFrom = oas.NewOptInt64(int64(op.Lt))
		}
	}
	return &res, nil
}

func (h *Handler) GetJettons(ctx context.Context, params oas.GetJettonsParams) (*oas.Jettons, error) {
	limit := 1000
	if params.Limit.IsSet() {
		limit = int(params.Limit.Value)
		if limit > 1000 || limit < 0 {
			limit = 1000
		}
	}
	offset := 0
	if params.Offset.IsSet() {
		offset = int(params.Offset.Value)
		if offset < 0 {
			offset = 0
		}
	}
	jettons, err := h.storage.GetJettonMasters(ctx, limit, offset)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	addresses := make([]tongo.AccountID, len(jettons))
	for idx, jetton := range jettons {
		addresses[idx] = jetton.Address
	}
	holders, err := h.storage.GetJettonsHoldersCount(ctx, addresses)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	results := make([]oas.JettonInfo, len(jettons))
	for idx, master := range jettons {
		scaledUiParams, err := h.storage.GetScaledUIParameters(ctx, master.Address, nil)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		results[idx] = h.convertJettonInfo(ctx, master, holders, scaledUiParams)
	}
	return &oas.Jettons{Jettons: results}, nil
}

func (h *Handler) GetJettonHolders(ctx context.Context, params oas.GetJettonHoldersParams) (*oas.JettonHolders, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	holders, err := h.storage.GetJettonHolders(ctx, account.ID, params.Limit.Value, params.Offset.Value)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	holderCounts, err := h.storage.GetJettonsHoldersCount(ctx, []tongo.AccountID{account.ID})
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.JettonHolders{}, nil
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	results := oas.JettonHolders{
		Addresses: make([]oas.JettonHoldersAddressesItem, 0, len(holders)),
		Total:     int64(holderCounts[account.ID]),
	}
	for _, holder := range holders {
		owner := NoneAccount
		if holder.Owner != nil {
			owner = convertAccountAddress(*holder.Owner, h.addressBook)
		}
		results.Addresses = append(results.Addresses, oas.JettonHoldersAddressesItem{
			Address: holder.Address.ToRaw(),
			Balance: holder.Balance.String(),
			Owner:   owner,
		})
	}
	return &results, nil
}

func (h *Handler) GetJettonsEvents(ctx context.Context, params oas.GetJettonsEventsParams) (*oas.Event, error) {
	traceID, err := tongo.ParseHash(params.EventID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	trace, err := h.storage.GetTrace(ctx, traceID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if errors.Is(err, core.ErrTraceIsTooLong) {
		return nil, toError(http.StatusRequestEntityTooLarge, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	result, err := bath.FindActions(ctx, trace, bath.WithInformationSource(h.storage), bath.WithStraws(bath.JettonTransfersBurnsMints))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	actionsList := bath.ActionsList{
		ValueFlow: &bath.ValueFlow{},
	}
	for _, item := range result.Actions {
		if item.Type != bath.JettonTransfer && item.Type != bath.JettonBurn && item.Type != bath.JettonMint {
			continue
		}
		actionsList.Actions = append(actionsList.Actions, item)
	}
	var response oas.Event
	if len(actionsList.Actions) == 0 {
		return nil, toError(http.StatusNotFound, fmt.Errorf("event %v has no interaction with jettons", params.EventID))
	}
	response, err = h.toEvent(ctx, trace, &actionsList, params.AcceptLanguage)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	isBannedTraces, err := h.spamFilter.GetEventsScamData(ctx, []string{traceID.Hex()})
	if err != nil {
		h.logger.Warn("error getting events spam data", zap.Error(err))
	}
	response.IsScam = response.IsScam || isBannedTraces[traceID.Hex()]
	return &response, nil
}

func (h *Handler) GetJettonTransferPayload(ctx context.Context, params oas.GetJettonTransferPayloadParams) (*oas.JettonTransferPayload, error) {
	accountID, err := ton.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	jettonMaster, err := ton.ParseAccountID(params.JettonID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	payload, err := h.storage.GetJettonTransferPayload(ctx, accountID, jettonMaster)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		if strings.Contains(err.Error(), "not implemented") {
			return nil, toError(http.StatusNotImplemented, err)
		}
		return nil, toError(http.StatusInternalServerError, err)
	}

	res := oas.JettonTransferPayload{}
	if payload.CustomPayload != nil {
		res.CustomPayload = oas.NewOptString(*payload.CustomPayload)
	}
	if payload.StateInit != nil {
		res.StateInit = oas.NewOptString(*payload.StateInit)
	}
	return &res, nil
}

func (h *Handler) GetJettonInfosByAddresses(ctx context.Context, request oas.OptGetJettonInfosByAddressesReq) (*oas.Jettons, error) {
	if len(request.Value.AccountIds) == 0 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("empty list of ids"))
	}
	if !h.limits.isBulkQuantityAllowed(len(request.Value.AccountIds)) {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("the maximum number of addresses to request at once: %v", h.limits.BulkLimits))
	}
	accounts := make([]ton.AccountID, len(request.Value.AccountIds))
	var err error
	for i := range request.Value.AccountIds {
		account, err := tongo.ParseAddress(request.Value.AccountIds[i])
		if err != nil {
			return nil, toError(http.StatusBadRequest, err)
		}
		accounts[i] = account.ID
	}
	jettons, err := h.storage.GetJettonMastersByAddresses(ctx, accounts)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	addresses := make([]tongo.AccountID, len(jettons))
	for idx, jetton := range jettons {
		addresses[idx] = jetton.Address
	}
	jettonsHolders, err := h.storage.GetJettonsHoldersCount(ctx, addresses)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	results := make([]oas.JettonInfo, len(jettons))
	for idx, master := range jettons {
		scaledUiParams, err := h.storage.GetScaledUIParameters(ctx, master.Address, nil)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		results[idx] = h.convertJettonInfo(ctx, master, jettonsHolders, scaledUiParams)
	}

	return &oas.Jettons{Jettons: results}, nil
}
