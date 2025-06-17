package api

import (
	"context"
	"crypto/ed25519"
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
	versions := getWalletAddressesByPubkey(publicKey)
	var addresses []ton.AccountID
	for addr, _ := range versions {
		addresses = append(addresses, addr)
	}
	rawAccounts, err := h.storage.GetRawAccounts(ctx, addresses)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	for i, _ := range rawAccounts {
		// TODO: or check for wallet interface
		if len(rawAccounts[i].Interfaces) == 0 {
			rawAccounts[i].Interfaces = append(rawAccounts[i].Interfaces, versions[rawAccounts[i].AccountAddress])
		}
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
	var (
		walletVersion abi.ContractInterface
		err           error
		sigAllowed    = true
	)
	for _, intr := range account.Interfaces {
		if _, ok := supported[intr]; ok {
			walletVersion = intr
			if intr == abi.WalletV5R1 {
				sigAllowed, err = h.storage.GetWalletSignatureAllowed(ctx, account.AccountAddress)
				if err != nil {
					return nil, err, http.StatusInternalServerError
				}
			}
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
		converted = convertToWallet(account, &ab, h.state, stats[0], plugins, sigAllowed)
	} else {
		converted = convertToWallet(account, nil, h.state, stats[0], plugins, sigAllowed)
	}
	return &converted, nil, http.StatusOK
}

var SupportedWallets = map[tongoWallet.Version]abi.ContractInterface{
	tongoWallet.V1R1:   abi.WalletV1R1,
	tongoWallet.V1R2:   abi.WalletV1R2,
	tongoWallet.V1R3:   abi.WalletV1R3,
	tongoWallet.V2R1:   abi.WalletV2R1,
	tongoWallet.V2R2:   abi.WalletV2R2,
	tongoWallet.V3R1:   abi.WalletV3R1,
	tongoWallet.V3R2:   abi.WalletV3R2,
	tongoWallet.V4R1:   abi.WalletV4R1,
	tongoWallet.V4R2:   abi.WalletV4R2,
	tongoWallet.V5Beta: abi.WalletV5Beta,
	tongoWallet.V5R1:   abi.WalletV5R1,
}

func getWalletAddressesByPubkey(pubKey ed25519.PublicKey) map[ton.AccountID]abi.ContractInterface {
	wallets := make(map[ton.AccountID]abi.ContractInterface, len(SupportedWallets))
	for version, ifc := range SupportedWallets {
		walletAddress, err := tongoWallet.GenerateWalletAddress(pubKey, version, nil, 0, nil)
		if err != nil {
			continue
		}
		wallets[walletAddress] = ifc
	}
	return wallets
}
