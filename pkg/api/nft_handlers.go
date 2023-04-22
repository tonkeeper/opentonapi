package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/tonkeeper/tongo"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func (h Handler) GetNftItemsByAddresses(ctx context.Context, req oas.OptGetNftItemsByAddressesReq) (oas.GetNftItemsByAddressesRes, error) {
	if len(req.Value.AccountIds) == 0 {
		return &oas.BadRequest{Error: "empty list of ids"}, nil
	}
	if !h.limits.isBulkQuantityAllowed(len(req.Value.AccountIds)) {
		return &oas.BadRequest{Error: fmt.Sprintf("the maximum number of addresses to request at once: %v", h.limits.BulkLimits)}, nil
	}
	accounts := make([]tongo.AccountID, len(req.Value.AccountIds))
	var err error
	for i := range req.Value.AccountIds {
		accounts[i], err = tongo.ParseAccountID(req.Value.AccountIds[i])
		if err != nil {
			return &oas.BadRequest{Error: err.Error()}, nil
		}
	}
	items, err := h.storage.GetNFTs(ctx, accounts)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	var result oas.NftItems
	for _, i := range items {
		result.NftItems = append(result.NftItems, convertNFT(ctx, i, h.addressBook, h.previewGenerator, h.metaCache))
	}
	return &result, nil
}

func (h Handler) GetNftItemByAddress(ctx context.Context, req oas.GetNftItemByAddressParams) (oas.GetNftItemByAddressRes, error) {
	account, err := tongo.ParseAccountID(req.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	items, err := h.storage.GetNFTs(ctx, []tongo.AccountID{account})
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	if len(items) != 1 {
		return &oas.NotFound{Error: "item not found"}, nil
	}
	result := convertNFT(ctx, items[0], h.addressBook, h.previewGenerator, h.metaCache)
	return &result, nil
}

func (h Handler) GetNftItemsByOwner(ctx context.Context, params oas.GetNftItemsByOwnerParams) (oas.GetNftItemsByOwnerRes, error) {
	account, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	ids, err := h.storage.SearchNFTs(ctx,
		nil,
		&core.Filter[tongo.AccountID]{Value: account},
		params.IndirectOwnership.Value,
		true,
		params.Limit.Value,
		params.Offset.Value)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	var result oas.NftItems
	if len(ids) == 0 {
		return &result, nil
	}
	items, err := h.storage.GetNFTs(ctx, ids)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	for _, i := range items {
		result.NftItems = append(result.NftItems, convertNFT(ctx, i, h.addressBook, h.previewGenerator, h.metaCache))
	}
	return &result, nil
}

func (h Handler) GetNftCollections(ctx context.Context, params oas.GetNftCollectionsParams) (oas.GetNftCollectionsRes, error) {
	collections, err := h.storage.GetNftCollections(ctx, &params.Limit.Value, &params.Offset.Value)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	var collectionsRes oas.NftCollections
	for _, collection := range collections {
		col := convertNftCollection(collection, h.addressBook)
		collectionsRes.NftCollections = append(collectionsRes.NftCollections, col)
	}
	return &collectionsRes, nil
}

func (h Handler) GetNftCollection(ctx context.Context, params oas.GetNftCollectionParams) (oas.GetNftCollectionRes, error) {
	account, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}

	collection, err := h.storage.GetNftCollectionByCollectionAddress(ctx, account)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.NotFound{Error: err.Error()}, nil
	}
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	col := convertNftCollection(collection, h.addressBook)
	return &col, nil
}

func (h Handler) GetItemsFromCollection(ctx context.Context, params oas.GetItemsFromCollectionParams) (oas.GetItemsFromCollectionRes, error) {
	account, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	ids, err := h.storage.SearchNFTs(ctx, &core.Filter[tongo.AccountID]{Value: account}, nil, false,
		true,
		params.Limit.Value, params.Offset.Value)
	var result oas.NftItems
	if len(ids) == 0 {
		return &result, nil
	}
	items, err := h.storage.GetNFTs(ctx, ids)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	for _, i := range items {
		result.NftItems = append(result.NftItems, convertNFT(ctx, i, h.addressBook, h.previewGenerator, h.metaCache))
	}
	return &result, nil
}
