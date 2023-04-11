package api

import (
	"context"
	"encoding/json"

	"github.com/tonkeeper/tongo"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
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
		if collection.Metadata == nil {
			collectionsRes.NftCollections = append(collectionsRes.NftCollections, col)
			continue
		}
		var metadata oas.OptNftCollectionMetadata
		err = json.Unmarshal(collection.Metadata, &metadata)
		if err == nil {
			col.Metadata = metadata
		}
		collectionsRes.NftCollections = append(collectionsRes.NftCollections, col)
	}
	return &collectionsRes, nil
}
