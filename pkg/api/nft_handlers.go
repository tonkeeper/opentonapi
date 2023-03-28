package api

import (
	"context"
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
