package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/wallet"
)

func (h Handler) SetWalletBackup(ctx context.Context, request oas.SetWalletBackupReq, params oas.SetWalletBackupParams) (res oas.SetWalletBackupRes, err error) {
	pubKey, verify, err := checkTonConnectToken(params.XTonConnectAuth, h.tonConnect.GetSecret())
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	if !verify {
		return &oas.BadRequest{Error: "failed verify"}, nil
	}

	walletBalance, err := getTotalBalances(ctx, h.storage, pubKey)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	if walletBalance < int64(tongo.OneTON) {
		return &oas.BadRequest{Error: "wallet must have more than 1 TON"}, nil
	}

	fileName := fmt.Sprintf("%x.dump", pubKey)
	tempFileName := fileName + fmt.Sprintf(".temp%v", time.Now().Nanosecond()+time.Now().Second())
	file, err := os.Create(tempFileName)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	defer file.Close()
	_, err = io.Copy(file, io.LimitReader(request.Data, 640*1024)) //640K ought to be enough for anybody
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	file.Close()
	err = os.Rename(tempFileName, fileName)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return &oas.SetWalletBackupOK{}, nil
}

func (h Handler) GetWalletBackup(ctx context.Context, params oas.GetWalletBackupParams) (res oas.GetWalletBackupRes, err error) {
	pubKey, verify, err := checkTonConnectToken(params.XTonConnectAuth, h.tonConnect.GetSecret())
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	if !verify {
		return &oas.BadRequest{Error: "failed verify"}, nil
	}

	dump, err := os.ReadFile(fmt.Sprintf("%v.dump", hex.EncodeToString(pubKey)))
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}

	return &oas.GetWalletBackupOK{Dump: string(dump)}, nil
}

func checkTonConnectToken(authToken, secret string) ([]byte, bool, error) {
	decodedData, err := base64.URLEncoding.DecodeString(authToken)
	if err != nil {
		return nil, false, err
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

	versions := []wallet.Version{
		wallet.V1R1, wallet.V1R2, wallet.V1R3,
		wallet.V2R1, wallet.V2R2,
		wallet.V3R1, wallet.V3R2,
		wallet.V4R1, wallet.V4R2,
	}

	var walletAddresses []tongo.AccountID
	for _, version := range versions {
		walletAddress, err := wallet.GenerateWalletAddress(pubKey, version, 0, nil)
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
