package api

import (
	"context"
	"errors"
	"math/big"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteapi"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func (h Handler) GetJettonsBalances(ctx context.Context, params oas.GetJettonsBalancesParams) (oas.GetJettonsBalancesRes, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	wallets, err := h.storage.GetJettonWalletsByOwnerAddress(ctx, accountID)
	if err != nil {
		if errors.Is(err, core.ErrEntityNotFound) {
			return &oas.NotFound{Error: err.Error()}, nil
		}
		return &oas.InternalError{Error: err.Error()}, nil
	}
	var balances = oas.JettonsBalances{
		Balances: make([]oas.JettonBalance, 0, len(wallets)),
	}
	for _, wallet := range wallets {
		jettonBalance := oas.JettonBalance{
			Balance:       wallet.Balance.String(),
			WalletAddress: convertAccountAddress(wallet.Address, h.addressBook),
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
			return &oas.InternalError{Error: err.Error()}, nil
		}
		var normalizedMetadata NormalizedMetadata
		info, ok := h.addressBook.GetJettonInfoByAddress(wallet.JettonAddress)
		if ok {
			normalizedMetadata = NormalizeMetadata(meta, &info)
		} else {
			normalizedMetadata = NormalizeMetadata(meta, nil)
		}
		jettonBalance.Jetton = jettonPreview(wallet.JettonAddress, normalizedMetadata, h.previewGenerator)
		balances.Balances = append(balances.Balances, jettonBalance)
	}

	return &balances, nil
}

func (h Handler) GetJettonInfo(ctx context.Context, params oas.GetJettonInfoParams) (r oas.GetJettonInfoRes, err error) {
	account, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	meta := h.GetJettonNormalizedMetadata(ctx, account)
	metadata := jettonMetadata(account, meta)
	data, err := h.storage.GetJettonMasterData(ctx, account)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	supply := big.Int(data.TotalSupply)
	return &oas.JettonInfo{
		Mintable:     data.Mintable != 0,
		TotalSupply:  supply.String(),
		Metadata:     metadata,
		Verification: oas.JettonVerificationType(meta.Verification),
	}, nil
}

func (h Handler) GetJettonsHistory(ctx context.Context, params oas.GetJettonsHistoryParams) (res oas.GetJettonsHistoryRes, err error) {
	account, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	traceIDs, err := h.storage.GetAccountJettonsHistory(ctx, account, params.Limit, optIntToPointer(params.BeforeLt), optIntToPointer(params.StartDate), optIntToPointer(params.EndDate))
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	events, lastLT, err := h.convertJettonHistory(ctx, account, nil, traceIDs, params.AcceptLanguage)
	return &oas.AccountEvents{Events: events, NextFrom: lastLT}, nil
}

func (h Handler) GetJettonsHistoryByID(ctx context.Context, params oas.GetJettonsHistoryByIDParams) (res oas.GetJettonsHistoryByIDRes, err error) {
	account, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	jettonMasterAccount, err := tongo.ParseAccountID(params.JettonID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	traceIDs, err := h.storage.GetAccountJettonHistoryByID(ctx, account, jettonMasterAccount, params.Limit, optIntToPointer(params.BeforeLt), optIntToPointer(params.StartDate), optIntToPointer(params.EndDate))
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	events, lastLT, err := h.convertJettonHistory(ctx, account, &jettonMasterAccount, traceIDs, params.AcceptLanguage)
	return &oas.AccountEvents{Events: events, NextFrom: lastLT}, nil
}

func (h Handler) GetJettons(ctx context.Context, params oas.GetJettonsParams) (r oas.GetJettonsRes, _ error) {
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
		return &oas.InternalError{Error: err.Error()}, nil
	}
	results := make([]oas.JettonInfo, 0, len(jettons))
	for _, master := range jettons {
		meta := h.GetJettonNormalizedMetadata(ctx, master.Address)
		metadata := jettonMetadata(master.Address, meta)
		info := oas.JettonInfo{
			Mintable:     master.Mintable,
			TotalSupply:  master.TotalSupply.String(),
			Metadata:     metadata,
			Verification: oas.JettonVerificationType(meta.Verification),
		}
		results = append(results, info)
	}
	return &oas.Jettons{
		Jettons: results,
	}, nil

}
