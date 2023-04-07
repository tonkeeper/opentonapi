package api

import (
	"context"
	"errors"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"math/big"
)

func (h Handler) GetJettonsBalances(ctx context.Context, params oas.GetJettonsBalancesParams) (oas.GetJettonsBalancesRes, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	wallets, err := h.storage.GetJettonWalletsByOwnerAddress(ctx, accountID)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	var balances = oas.JettonsBalances{
		Balances: make([]oas.JettonBalance, 0, len(wallets)),
	}
	for _, wallet := range wallets {
		jettonBalance := oas.JettonBalance{
			Balance:       wallet.Balance.String(),
			WalletAddress: convertAccountAddress(wallet.Address),
		}
		meta, err := h.storage.GetJettonMasterMetadata(ctx, wallet.JettonAddress)
		if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
			return &oas.InternalError{Error: err.Error()}, nil
		}
		jettonBalance.Jetton = jettonPreview(h.addressBook, wallet.JettonAddress, meta, h.previewGenerator)
		balances.Balances = append(balances.Balances, jettonBalance)
	}

	return &balances, nil
}

func (h Handler) GetJettonInfo(ctx context.Context, params oas.GetJettonInfoParams) (r oas.GetJettonInfoRes, err error) {
	verification := oas.JettonVerificationTypeNone
	account, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	meta, err := h.storage.GetJettonMasterMetadata(ctx, account)
	metadata := oas.JettonMetadata{Address: account.ToRaw()}
	info, ok := h.addressBook.GetJettonInfoByAddress(account)
	if ok {
		meta.Name = rewriteIfNotEmpty(meta.Name, info.Name)
		meta.Description = rewriteIfNotEmpty(meta.Description, info.Description)
		meta.Image = rewriteIfNotEmpty(meta.Image, info.Image)
		meta.Symbol = rewriteIfNotEmpty(meta.Symbol, info.Symbol)
		verification = oas.JettonVerificationTypeWhitelist
	}
	metadata.Name = meta.Name
	metadata.Symbol = meta.Symbol
	metadata.Decimals = meta.Decimals
	metadata.Social = info.Social
	metadata.Websites = info.Websites

	if meta.Description != "" {
		metadata.Description.SetTo(meta.Description)
	}
	if meta.Image != "" {
		metadata.Image.SetTo(meta.Image)
	}
	data, err := h.storage.GetJettonMasterData(ctx, account)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	supply := big.Int(data.TotalSupply)
	return &oas.JettonInfo{
		Mintable:     data.Mintable != 0,
		TotalSupply:  supply.String(),
		Metadata:     metadata,
		Verification: verification,
	}, nil
}
