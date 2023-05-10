package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/wallet"
)

const (
	dirPath = "/opt/wallets_backup" // TODO: change to s3
)

func (h Handler) SetWalletBackup(ctx context.Context, request oas.OptSetWalletBackupReq, params oas.SetWalletBackupParams) (res oas.SetWalletBackupRes, err error) {
	pubKey, verify, err := checkTonConnectToken(params.XTonConnectAuth, h.tonConnect.GetSecret())
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	if !verify {
		return &oas.BadRequest{Error: "failed verify"}, nil
	}

	fileName := hex.EncodeToString(pubKey)

	walletBalance, err := getTotalBalances(ctx, h.storage, pubKey)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	if walletBalance < int64(tongo.OneTON) {
		return &oas.BadRequest{Error: "wallet must have more than 1 TON"}, nil
	}

	if _, err = os.Stat(dirPath); os.IsNotExist(err) {
		err = os.MkdirAll(dirPath, 0755)
		if err != nil {
			return &oas.InternalError{Error: err.Error()}, nil
		}
	}

	bytesData, err := request.Value.MarshalJSON()
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}

	filePath := fmt.Sprintf("%v/%v.dump", dirPath, fileName)
	err = ioutil.WriteFile(filePath, bytesData, 0644)
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

	fileName := hex.EncodeToString(pubKey)

	if _, err = os.Stat(dirPath); os.IsNotExist(err) {
		err = os.MkdirAll(dirPath, 0755)
		if err != nil {
			return &oas.InternalError{Error: err.Error()}, nil
		}
	}

	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}

	var dump string
	for _, file := range files {
		if file.Name() == fileName {
			filePath := filepath.Join(dirPath, fmt.Sprintf("%v.dump", fileName))
			data, err := ioutil.ReadFile(filePath)
			if err != nil {
				return &oas.InternalError{Error: err.Error()}, nil
			}
			dump = string(data)
		}
	}

	return &oas.GetWalletBackupOK{Dump: dump}, nil
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
