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
)

func (h *Handler) GetAccountJettonsBalances(ctx context.Context, params oas.GetAccountJettonsBalancesParams) (*oas.JettonsBalances, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	wallets, err := h.storage.GetJettonWalletsByOwnerAddress(ctx, account.ID, nil)
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
		if slices.Contains(wallet.Extensions, "custom_payload") && !slices.Contains(params.SupportedExtensions, "custom_payload") {
			continue
		}
		jettonBalance, err := h.convertJettonBalance(ctx, wallet, params.Currencies)
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
	wallets, err := h.storage.GetJettonWalletsByOwnerAddress(ctx, account.ID, &jettonAccount.ID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.JettonBalance{}, nil
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	if len(wallets) == 0 {
		return &oas.JettonBalance{}, nil
	}
	jettonBalance, err := h.convertJettonBalance(ctx, wallets[0], params.Currencies)
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
	meta := h.GetJettonNormalizedMetadata(ctx, account.ID)
	metadata := jettonMetadata(account.ID, meta)
	data, err := h.storage.GetJettonMasterData(ctx, account.ID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	holdersCount, err := h.storage.GetJettonsHoldersCount(ctx, []tongo.AccountID{account.ID})
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &oas.JettonInfo{
		Mintable:     data.Mintable,
		TotalSupply:  data.TotalSupply.String(),
		Metadata:     metadata,
		Verification: oas.JettonVerificationType(meta.Verification),
		HoldersCount: holdersCount[account.ID],
		Admin:        convertOptAccountAddress(data.Admin, h.addressBook),
	}, nil
}

func (h *Handler) GetAccountJettonsHistory(ctx context.Context, params oas.GetAccountJettonsHistoryParams) (*oas.AccountEvents, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	traceIDs, err := h.storage.GetAccountJettonsHistory(ctx, account.ID, params.Limit, optIntToPointer(params.BeforeLt), optIntToPointer(params.StartDate), optIntToPointer(params.EndDate))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	events, lastLT, err := h.convertJettonHistory(ctx, account.ID, nil, traceIDs, params.AcceptLanguage)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &oas.AccountEvents{Events: events, NextFrom: lastLT}, nil
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
	events, lastLT, err := h.convertJettonHistory(ctx, account.ID, &jettonMasterAccount.ID, traceIDs, params.AcceptLanguage)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &oas.AccountEvents{Events: events, NextFrom: lastLT}, nil
}

func (h *Handler) GetJettons(ctx context.Context, params oas.GetJettonsParams) (*oas.Jettons, error) {
	limit := 1000
	offset := 0
	if params.Limit.IsSet() {
		limit = int(params.Limit.Value)
	}
	if limit > 1000 {
		limit = 1000
	}
	if limit < 0 {
		limit = 1000
	}
	if params.Offset.IsSet() {
		offset = int(params.Offset.Value)
	}
	if offset < 0 {
		offset = 0
	}
	jettons, err := h.storage.GetJettonMasters(ctx, limit, offset)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	results := make([]oas.JettonInfo, 0, len(jettons))
	var addresses []tongo.AccountID
	for _, jetton := range jettons {
		addresses = append(addresses, jetton.Address)
	}
	jettonsHolders, err := h.storage.GetJettonsHoldersCount(ctx, addresses)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	for _, master := range jettons {
		meta := h.GetJettonNormalizedMetadata(ctx, master.Address)
		metadata := jettonMetadata(master.Address, meta)
		info := oas.JettonInfo{
			Mintable:     master.Mintable,
			TotalSupply:  master.TotalSupply.String(),
			Metadata:     metadata,
			Verification: oas.JettonVerificationType(meta.Verification),
			HoldersCount: jettonsHolders[master.Address],
			Admin:        convertOptAccountAddress(master.Admin, h.addressBook),
		}
		results = append(results, info)
	}
	return &oas.Jettons{
		Jettons: results,
	}, nil
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
		results.Addresses = append(results.Addresses, oas.JettonHoldersAddressesItem{
			Address: holder.Address.ToRaw(),
			Owner:   convertAccountAddress(holder.Owner, h.addressBook),
			Balance: holder.Balance.String(),
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
