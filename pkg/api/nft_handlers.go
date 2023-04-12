package api

import (
	"context"
	"errors"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
)

func (h Handler) GetNftItemsByAddresses(ctx context.Context, req oas.OptGetNftItemsByAddressesReq) (oas.GetNftItemsByAddressesRes, error) {
	if len(req.Value.AccountIds) == 0 {
		return &oas.BadRequest{Error: "empty list of ids"}, nil
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
		result.NftItems = append(result.NftItems, convertNFT(i, h.addressBook, h.previewGenerator))
	}
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
		false, //todo: enable after reindexing
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
		result.NftItems = append(result.NftItems, convertNFT(i, h.addressBook, h.previewGenerator))
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
