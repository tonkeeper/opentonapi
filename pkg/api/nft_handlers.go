package api

import (
	"context"
	"encoding/json"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
)

func (h Handler) GetNftItemsByAddresses(ctx context.Context, params oas.GetNftItemsByAddressesParams) (oas.GetNftItemsByAddressesRes, error) {
	accounts := make([]tongo.AccountID, len(params.AccountIds))
	var err error
	for i := range params.AccountIds {
		accounts[i], err = tongo.ParseAccountID(params.AccountIds[i])
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
		result.NftItems = append(result.NftItems, convertNFT(i))
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
		result.NftItems = append(result.NftItems, convertNFT(i))
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
		col := convertNftCollection(collection)
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
