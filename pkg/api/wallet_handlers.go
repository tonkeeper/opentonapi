package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/core"

	"github.com/tonkeeper/tongo/ton"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/wallet"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	tongoWallet "github.com/tonkeeper/tongo/wallet"
)

func (h *Handler) SetWalletBackup(ctx context.Context, request oas.SetWalletBackupReq, params oas.SetWalletBackupParams) error {
	pubKey, verify, err := checkTonConnectToken(params.XTonConnectAuth, h.tonConnect.GetSecret())
	if err != nil {
		return toError(http.StatusBadRequest, err)
	}
	if !verify {
		return toError(http.StatusBadRequest, fmt.Errorf("failed verify"))
	}

	walletBalance, err := getTotalBalances(ctx, h.storage, pubKey)
	if err != nil {
		return toError(http.StatusInternalServerError, err)
	}
	if walletBalance < int64(ton.OneTON) {
		return toError(http.StatusBadRequest, fmt.Errorf("wallet must have more than 1 TON"))
	}

	fileName := fmt.Sprintf("%x.dump", pubKey)
	tempFileName := fileName + fmt.Sprintf(".temp%v", time.Now().Nanosecond()+time.Now().Second())
	file, err := os.Create(tempFileName)
	if err != nil {
		return toError(http.StatusInternalServerError, err)
	}
	defer file.Close()
	_, err = io.Copy(file, io.LimitReader(request.Data, 640*1024)) //640K ought to be enough for anybody
	if err != nil {
		return toError(http.StatusInternalServerError, err)
	}
	file.Close()
	err = os.Rename(tempFileName, fileName)
	if err != nil {
		return toError(http.StatusInternalServerError, err)
	}
	return nil
}

func (h *Handler) GetWalletBackup(ctx context.Context, params oas.GetWalletBackupParams) (*oas.GetWalletBackupOK, error) {
	pubKey, verify, err := checkTonConnectToken(params.XTonConnectAuth, h.tonConnect.GetSecret())
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	if !verify {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("failed verify"))
	}

	dump, err := os.ReadFile(fmt.Sprintf("%v.dump", hex.EncodeToString(pubKey)))
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}

	return &oas.GetWalletBackupOK{Dump: string(dump)}, nil
}

func checkTonConnectToken(authToken, secret string) ([]byte, bool, error) {
	decodedData, err := base64.URLEncoding.DecodeString(authToken)
	if err != nil {
		return nil, false, err
	}
	if len(decodedData) <= 32 {
		return nil, false, fmt.Errorf("invalid payload length")
	}
	pubKey := decodedData[:32]
	signature := decodedData[32:]

	hmacHash := hmac.New(sha256.New, []byte(secret))
	hmacHash.Write(pubKey)
	computedSignature := hmacHash.Sum(nil)
	if !hmac.Equal(signature, computedSignature) {
		return nil, false, nil
	}

	return pubKey, true, nil
}

func getTotalBalances(ctx context.Context, storage storage, pubKey []byte) (int64, error) {
	var balance int64
	versions := []tongoWallet.Version{
		tongoWallet.V1R1, tongoWallet.V1R2, tongoWallet.V1R3,
		tongoWallet.V2R1, tongoWallet.V2R2,
		tongoWallet.V3R1, tongoWallet.V3R2,
		tongoWallet.V4R1, tongoWallet.V4R2,
		tongoWallet.V5Beta,
	}
	var walletAddresses []tongo.AccountID
	for _, version := range versions {
		walletAddress, err := tongoWallet.GenerateWalletAddress(pubKey, version, nil, 0, nil)
		if err != nil {
			continue
		}
		walletAddresses = append(walletAddresses, walletAddress)
	}
	for _, address := range walletAddresses {
		account, err := storage.GetRawAccount(ctx, address)
		if err != nil {
			continue
		}
		balance += account.TonBalance
	}
	return balance, nil
}

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
