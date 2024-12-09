package api

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"net/http"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/wallet"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	tongoWallet "github.com/tonkeeper/tongo/wallet"
)

func (h *Handler) GetWalletsByPublicKey(ctx context.Context, params oas.GetWalletsByPublicKeyParams) (*oas.Accounts, error) {
	publicKey, err := hex.DecodeString(params.PublicKey)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	walletAddresses, err := h.storage.SearchAccountsByPubKey(ctx, publicKey)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	accounts, err := h.storage.GetRawAccounts(ctx, walletAddresses)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	results := make([]oas.Account, 0, len(accounts))
	for _, account := range accounts {
		ab, found := h.addressBook.GetAddressInfoByAddress(account.AccountAddress)
		var res oas.Account
		if found {
			res = convertToAccount(account, &ab, h.state)
		} else {
			res = convertToAccount(account, nil, h.state)
		}
		if account.ExtraBalances != nil {
			res.ExtraBalance = convertExtraCurrencies(account.ExtraBalances)
		}
		results = append(results, res)
	}
	return &oas.Accounts{Accounts: results}, nil
}

func (h *Handler) GetAccountSeqno(ctx context.Context, params oas.GetAccountSeqnoParams) (*oas.Seqno, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	var seqno uint32
	seqno, err = h.storage.GetSeqno(ctx, account.ID)
	if err == nil {
		return &oas.Seqno{Seqno: int32(seqno)}, nil
	}
	rawAccount, err := h.storage.GetRawAccount(ctx, account.ID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.Seqno{Seqno: int32(seqno)}, nil
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	if len(rawAccount.Code) == 0 {
		return &oas.Seqno{Seqno: int32(seqno)}, nil
	}
	walletVersion, err := wallet.GetVersionByCode(rawAccount.Code)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	cells, err := boc.DeserializeBoc(rawAccount.Data)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}

	switch walletVersion {
	case tongoWallet.V1R1, tongoWallet.V1R2, tongoWallet.V1R3:
		var data tongoWallet.DataV1V2
		err = tlb.Unmarshal(cells[0], &data)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		seqno = data.Seqno
	case tongoWallet.V3R1:
		var data tongoWallet.DataV3
		err = tlb.Unmarshal(cells[0], &data)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		seqno = data.Seqno
	default:
		return nil, toError(http.StatusBadRequest, fmt.Errorf("contract doesn't have a seqno"))
	}
	return &oas.Seqno{Seqno: int32(seqno)}, nil
}
