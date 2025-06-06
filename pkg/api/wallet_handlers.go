package api

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/wallet"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	tongoWallet "github.com/tonkeeper/tongo/wallet"
)

var errUnsupportedWalletVersion = errors.New("unsupported wallet version")

func (h *Handler) GetWalletsByPublicKey(ctx context.Context, params oas.GetWalletsByPublicKeyParams) (*oas.Wallets, error) {
	publicKey, err := hex.DecodeString(params.PublicKey)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	walletAddresses, err := h.storage.SearchAccountsByPubKey(ctx, publicKey)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	rawAccounts, err := h.storage.GetRawAccounts(ctx, walletAddresses)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	wallets := make([]oas.Wallet, 0, len(rawAccounts))
	for _, rawAccount := range rawAccounts {
		converted, err, statusCode := h.processWallet(ctx, rawAccount)
		if errors.Is(err, errUnsupportedWalletVersion) {
			continue
		}
		if err != nil {
			return nil, toError(statusCode, err)
		}
		wallets = append(wallets, *converted)
	}
	return &oas.Wallets{Accounts: wallets}, nil
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

func (h *Handler) GetWalletInfo(ctx context.Context, params oas.GetWalletInfoParams) (*oas.Wallet, error) {
	account, err := tongo.ParseAddress(params.AccountID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	rawAccount, err := h.storage.GetRawAccount(ctx, account.ID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	converted, err, statusCode := h.processWallet(ctx, rawAccount)
	if err != nil {
		return nil, toError(statusCode, err)
	}
	return converted, nil
}

func (h *Handler) processWallet(ctx context.Context, account *core.Account) (*oas.Wallet, error, int) {
	supported := map[abi.ContractInterface]struct{}{
		abi.WalletV3R1: {}, abi.WalletV3R2: {},
		abi.WalletV4R1: {}, abi.WalletV4R2: {},
		abi.WalletV5Beta: {}, abi.WalletV5R1: {},
	}
	var walletVersion abi.ContractInterface
	for _, intr := range account.Interfaces {
		if _, ok := supported[intr]; ok {
			walletVersion = intr
			break
		}
	}
	if walletVersion == 0 {
		return nil, errUnsupportedWalletVersion, http.StatusBadRequest
	}
	stats, err := h.storage.GetAccountsStats(ctx, []ton.AccountID{account.AccountAddress})
	if err != nil {
		return nil, err, http.StatusInternalServerError
	}
	if len(stats) == 0 {
		return nil, errors.New("account not found"), http.StatusNotFound
	}
	plugins, err := h.storage.GetAccountPlugins(ctx, account.AccountAddress, walletVersion)
	if err != nil {
		return nil, err, http.StatusInternalServerError
	}
	var converted oas.Wallet
	ab, found := h.addressBook.GetAddressInfoByAddress(account.AccountAddress)
	if found {
		converted = convertToWallet(account, &ab, h.state, stats[0], plugins)
	} else {
		converted = convertToWallet(account, nil, h.state, stats[0], plugins)
	}
	return &converted, nil, http.StatusOK
}
