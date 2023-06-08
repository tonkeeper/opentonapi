package api

import (
	"context"
	"errors"
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/core/jetton"
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
		var normalizedMetadata jetton.NormalizedMetadata
		info, ok := h.addressBook.GetJettonInfoByAddress(wallet.JettonAddress)
		if ok {
			normalizedMetadata = jetton.NormalizeMetadata(meta, &info)
		} else {
			normalizedMetadata = jetton.NormalizeMetadata(meta, nil)
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
	events, lastLT, err := h.convertJettonHistory(ctx, account, traceIDs, params.AcceptLanguage)
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
	events, lastLT, err := h.convertJettonHistory(ctx, account, traceIDs, params.AcceptLanguage)
	return &oas.AccountEvents{Events: events, NextFrom: lastLT}, nil
}
