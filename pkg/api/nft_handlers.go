package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/tonkeeper/tongo"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func (h *Handler) GetNftItemsByAddresses(ctx context.Context, request oas.OptGetNftItemsByAddressesReq) (*oas.NftItems, error) {
	if len(request.Value.AccountIds) == 0 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("empty list of ids"))
	}
	if !h.limits.isBulkQuantityAllowed(len(request.Value.AccountIds)) {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("the maximum number of addresses to request at once: %v", h.limits.BulkLimits))
	}
	accounts := make([]tongo.AccountID, len(request.Value.AccountIds))
	var err error
	for i := range request.Value.AccountIds {
		accounts[i], err = tongo.ParseAccountID(request.Value.AccountIds[i])
		if err != nil {
			return nil, toError(http.StatusBadRequest, err)
		}
	}
	items, err := h.storage.GetNFTs(ctx, accounts)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var result oas.NftItems
	for _, i := range items {
		result.NftItems = append(result.NftItems, convertNFT(ctx, i, h.addressBook, h.metaCache))
	}
	return &result, nil
}

func (h *Handler) GetNftItemByAddress(ctx context.Context, params oas.GetNftItemByAddressParams) (*oas.NftItem, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	items, err := h.storage.GetNFTs(ctx, []tongo.AccountID{accountID})
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	if len(items) != 1 {
		return nil, toError(http.StatusNotFound, fmt.Errorf("item not found"))
	}
	result := convertNFT(ctx, items[0], h.addressBook, h.metaCache)
	return &result, nil
}

func (h *Handler) GetAccountNftItems(ctx context.Context, params oas.GetAccountNftItemsParams) (*oas.NftItems, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var collectionFilter *core.Filter[tongo.AccountID]
	if params.Collection.Value != "" {
		collection, err := tongo.ParseAccountID(params.Collection.Value)
		if err != nil {
			return nil, toError(http.StatusBadRequest, err)
		}
		collectionFilter = &core.Filter[tongo.AccountID]{Value: collection}
	}
	ids, err := h.storage.SearchNFTs(ctx,
		collectionFilter,
		&core.Filter[tongo.AccountID]{Value: accountID},
		params.IndirectOwnership.Value,
		true,
		params.Limit.Value,
		params.Offset.Value)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var result oas.NftItems
	if len(ids) == 0 {
		return &result, nil
	}
	items, err := h.storage.GetNFTs(ctx, ids)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	for _, i := range items {
		result.NftItems = append(result.NftItems, convertNFT(ctx, i, h.addressBook, h.metaCache))
	}
	return &result, nil
}

func (h *Handler) GetNftCollections(ctx context.Context, params oas.GetNftCollectionsParams) (*oas.NftCollections, error) {
	collections, err := h.storage.GetNftCollections(ctx, &params.Limit.Value, &params.Offset.Value)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var collectionsRes oas.NftCollections
	for _, collection := range collections {
		col := convertNftCollection(collection, h.addressBook)
		collectionsRes.NftCollections = append(collectionsRes.NftCollections, col)
	}
	return &collectionsRes, nil
}

func (h *Handler) GetNftCollection(ctx context.Context, params oas.GetNftCollectionParams) (*oas.NftCollection, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	collection, err := h.storage.GetNftCollectionByCollectionAddress(ctx, accountID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	col := convertNftCollection(collection, h.addressBook)
	return &col, nil
}

func (h *Handler) GetItemsFromCollection(ctx context.Context, params oas.GetItemsFromCollectionParams) (*oas.NftItems, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	ids, err := h.storage.SearchNFTs(ctx, &core.Filter[tongo.AccountID]{Value: accountID}, nil, false,
		true,
		params.Limit.Value, params.Offset.Value)
	var result oas.NftItems
	if len(ids) == 0 {
		return &result, nil
	}
	items, err := h.storage.GetNFTs(ctx, ids)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	for _, i := range items {
		result.NftItems = append(result.NftItems, convertNFT(ctx, i, h.addressBook, h.metaCache))
	}
	return &result, nil
}

func (h *Handler) GetNftHistoryByID(ctx context.Context, params oas.GetNftHistoryByIDParams) (*oas.AccountEvents, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	traceIDs, err := h.storage.GetNftHistory(ctx, accountID, params.Limit, optIntToPointer(params.BeforeLt), optIntToPointer(params.StartDate), optIntToPointer(params.EndDate))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	events, lastLT, err := h.convertNftHistory(ctx, accountID, traceIDs, params.AcceptLanguage)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return &oas.AccountEvents{Events: events, NextFrom: lastLT}, nil
}
