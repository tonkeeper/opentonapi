package api

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/ton"

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
			res = convertToAccount(account, &ab, h.state, h.spamFilter)
		} else {
			res = convertToAccount(account, nil, h.state, h.spamFilter)
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
	supportedWalletVersions := map[abi.ContractInterface]struct{}{
		abi.WalletV4R1: {}, abi.WalletV4R2: {},
		abi.WalletV5Beta: {}, abi.WalletV5R1: {},
	}
	var walletVersion abi.ContractInterface
	for _, intr := range rawAccount.Interfaces {
		if _, ok := supportedWalletVersions[intr]; !ok {
			return nil, toError(http.StatusBadRequest, fmt.Errorf("unsupported account"))
		}
		walletVersion = intr
	}
	ab, found := h.addressBook.GetAddressInfoByAddress(account.ID)
	var convertedAccountInfo oas.Account
	if found {
		convertedAccountInfo = convertToAccount(rawAccount, &ab, h.state, h.spamFilter)
	} else {
		convertedAccountInfo = convertToAccount(rawAccount, nil, h.state, h.spamFilter)
	}
	accountPlugins, err := h.storage.GetAccountPlugins(ctx, account.ID, walletVersion)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	stats, err := h.storage.GetAccountsStats(ctx, []ton.AccountID{account.ID})
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	if len(stats) == 0 {
		return nil, toError(http.StatusNotFound, fmt.Errorf("account not found"))
	}
	result := oas.Wallet{
		Address:     account.ID.ToRaw(),
		Name:        convertedAccountInfo.Name,
		Icon:        convertedAccountInfo.Icon,
		IsSuspended: convertedAccountInfo.IsSuspended,
		Stats: oas.WalletStats{
			TonBalance:    stats[0].TonBalance,
			NftsCount:     stats[0].NftsCount,
			JettonsCount:  stats[0].JettonsCount,
			MultisigCount: stats[0].MultisigCount,
			StakingCount:  stats[0].StakingCount,
		},
	}
	for _, plugin := range accountPlugins {
		result.Plugins = append(result.Plugins, oas.WalletPlugin{
			Address: plugin.AccountID.ToRaw(),
			Type:    plugin.Type,
		})
	}

	return &result, nil
}
