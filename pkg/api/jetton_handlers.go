package api

import (
	"context"
	"errors"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
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
			JettonAddress: wallet.JettonAddress.ToRaw(),
			WalletAddress: convertAccountAddress(wallet.Address),
		}
		meta, err := h.storage.GetJettonMasterMetadata(ctx, wallet.JettonAddress)
		if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
			return &oas.InternalError{Error: err.Error()}, nil
		}
		if errors.Is(err, core.ErrEntityNotFound) {
			meta = tongo.JettonMetadata{
				Name:  "Unknown",
				Image: "https://ton.ams3.digitaloceanspaces.com/token-placeholder-288.png",
			}
		}
		info, ok := h.addressBook.GetJettonInfoByAddress(wallet.JettonAddress)
		if ok {
			meta.Name = rewriteIfNotEmpty(meta.Name, info.Name)
			meta.Description = rewriteIfNotEmpty(meta.Description, info.Description)
			meta.Image = rewriteIfNotEmpty(meta.Image, info.Image)
			meta.Symbol = rewriteIfNotEmpty(meta.Symbol, info.Symbol)
		}
		m, err := convertToApiJetton(meta, wallet.JettonAddress, h.previewGenerator)
		if err != nil {
			return &oas.InternalError{Error: err.Error()}, nil
		}
		m.Verification = oas.OptJettonVerificationType{Value: oas.JettonVerificationTypeNone}
		jettonBalance.Metadata = oas.OptJetton{Value: m}
		convertVerification, _ := convertJettonVerification(addressbook.None) // TODO: change to real verify
		jettonBalance.Verification = convertVerification

		balances.Balances = append(balances.Balances, jettonBalance)
	}

	return &balances, nil
}
