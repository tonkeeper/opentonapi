package api

import (
	"context"
	"errors"
	"math/big"
	"net/http"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteapi"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func (h Handler) GetAccountJettonsBalances(ctx context.Context, params oas.GetAccountJettonsBalancesParams) (*oas.JettonsBalances, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	wallets, err := h.storage.GetJettonWalletsByOwnerAddress(ctx, accountID)
	if err != nil {
		if errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusNotFound, err)
		}
		return nil, toError(http.StatusInternalServerError, err)
	}
	var balances = oas.JettonsBalances{
		Balances: make([]oas.JettonBalance, 0, len(wallets)),
	}
	for _, wallet := range wallets {
		jettonBalance := oas.JettonBalance{
			Balance:       wallet.Balance.String(),
			WalletAddress: convertAccountAddress(wallet.Address, h.addressBook, h.previewGenerator),
		}
		meta, err := h.storage.GetJettonMasterMetadata(ctx, wallet.JettonAddress)
		if err != nil && err.Error() == "not enough refs" {
			// happens when metadata is broken, for example.
			continue
		}
		if err != nil && errors.Is(err, liteapi.ErrOnchainContentOnly) {
			// we don't support such jettons
			continue
		}
		if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusNotFound, err)
		}
		var normalizedMetadata NormalizedMetadata
		info, ok := h.addressBook.GetJettonInfoByAddress(wallet.JettonAddress)
		if ok {
			normalizedMetadata = NormalizeMetadata(meta, &info, h.previewGenerator)
		} else {
			normalizedMetadata = NormalizeMetadata(meta, nil, h.previewGenerator)
		}
		jettonBalance.Jetton = jettonPreview(wallet.JettonAddress, normalizedMetadata)
		balances.Balances = append(balances.Balances, jettonBalance)
	}

	return &balances, nil
}

func (h Handler) GetJettonInfo(ctx context.Context, params oas.GetJettonInfoParams) (*oas.JettonInfo, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	meta := h.GetJettonNormalizedMetadata(ctx, accountID, h.previewGenerator)
	metadata := jettonMetadata(accountID, meta)
	data, err := h.storage.GetJettonMasterData(ctx, accountID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	holdersCount, err := h.storage.GetJettonsHoldersCount(ctx, []tongo.AccountID{accountID})
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	supply := big.Int(data.TotalSupply)
	return &oas.JettonInfo{
		Mintable:     data.Mintable != 0,
		TotalSupply:  supply.String(),
		Metadata:     metadata,
		Verification: oas.JettonVerificationType(meta.Verification),
		HoldersCount: holdersCount[accountID],
	}, nil
}

func (h Handler) GetAccountJettonsHistory(ctx context.Context, params oas.GetAccountJettonsHistoryParams) (*oas.AccountEvents, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	traceIDs, err := h.storage.GetAccountJettonsHistory(ctx, accountID, params.Limit, optIntToPointer(params.BeforeLt), optIntToPointer(params.StartDate), optIntToPointer(params.EndDate))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	events, lastLT, err := h.convertJettonHistory(ctx, accountID, nil, traceIDs, params.AcceptLanguage)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &oas.AccountEvents{Events: events, NextFrom: lastLT}, nil
}

func (h Handler) GetAccountJettonHistoryByID(ctx context.Context, params oas.GetAccountJettonHistoryByIDParams) (*oas.AccountEvents, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	jettonMasterAccountID, err := tongo.ParseAccountID(params.JettonID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	traceIDs, err := h.storage.GetAccountJettonHistoryByID(ctx, accountID, jettonMasterAccountID, params.Limit, optIntToPointer(params.BeforeLt), optIntToPointer(params.StartDate), optIntToPointer(params.EndDate))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	events, lastLT, err := h.convertJettonHistory(ctx, accountID, &jettonMasterAccountID, traceIDs, params.AcceptLanguage)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &oas.AccountEvents{Events: events, NextFrom: lastLT}, nil
}

func (h Handler) GetJettons(ctx context.Context, params oas.GetJettonsParams) (*oas.Jettons, error) {
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
		meta := h.GetJettonNormalizedMetadata(ctx, master.Address, h.previewGenerator)
		metadata := jettonMetadata(master.Address, meta)
		info := oas.JettonInfo{
			Mintable:     master.Mintable,
			TotalSupply:  master.TotalSupply.String(),
			Metadata:     metadata,
			Verification: oas.JettonVerificationType(meta.Verification),
			HoldersCount: jettonsHolders[master.Address],
		}
		results = append(results, info)
	}
	return &oas.Jettons{
		Jettons: results,
	}, nil
}

func (h Handler) GetJettonHolders(ctx context.Context, params oas.GetJettonHoldersParams) (*oas.JettonHolders, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	holders, err := h.storage.GetJettonHolders(ctx, accountID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var results oas.JettonHolders
	for _, holder := range holders {
		results.Addresses = append(results.Addresses, oas.JettonHoldersAddressesItem{
			Address: holder.Address.ToRaw(),
			Balance: holder.Balance.String(),
		})
	}
	return &results, nil
}
