package litestorage

import (
	"context"
	"fmt"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

func (s *LiteStorage) GetJettonWalletsByOwnerAddress(ctx context.Context, address tongo.AccountID) ([]core.JettonWallet, error) {
	wallets := []core.JettonWallet{}

	for _, jetton := range s.knownAccounts["jettons"] {
		_, result, err := abi.GetWalletAddress(ctx, s.client, jetton, address.ToMsgAddress())
		if err != nil {
			continue
		}
		walletAddress := result.(abi.GetWalletAddressResult)
		jettonAccountID, err := tongo.AccountIDFromTlb(walletAddress.JettonWalletAddress)
		if err != nil {
			continue
		}
		_, result, err = abi.GetWalletData(ctx, s.client, *jettonAccountID)
		if err != nil {
			continue
		}
		jettonWallet := result.(core.JettonWallet)
		if jettonWallet.Address != jetton {
			continue
		}

		wallets = append(wallets, jettonWallet)
	}

	return wallets, nil
}

func (s *LiteStorage) GetJettonMasterMetadata(ctx context.Context, master tongo.AccountID) (tongo.JettonMetadata, error) {
	meta, ok := s.jettonMetaCache[master.ToRaw()]
	if ok {
		return meta, nil
	}
	rawMeta, err := s.client.GetJettonData(ctx, master)
	if err != nil {
		return tongo.JettonMetadata{}, err
	}
	s.jettonMetaCache[master.ToRaw()] = rawMeta
	return rawMeta, nil
}

func (s *LiteStorage) GetJettonMasterData(ctx context.Context, master tongo.AccountID) (abi.GetJettonDataResult, error) {
	_, value, err := abi.GetJettonData(ctx, s.client, master)
	if err != nil {
		return abi.GetJettonDataResult{}, err
	}
	r, ok := value.(abi.GetJettonDataResult)
	if !ok {
		return abi.GetJettonDataResult{}, fmt.Errorf("invalid jetton data result")
	}
	return r, nil
}

func (s *LiteStorage) GetAccountJettonsHistory(ctx context.Context, address tongo.AccountID, limit int, beforeLT *int64, startTime *int64, endTime *int64) ([]tongo.Bits256, error) {
	return nil, nil
}

func (s *LiteStorage) GetAccountJettonHistoryByID(ctx context.Context, address, jettonMaster tongo.AccountID, limit int, beforeLT *int64, startTime *int64, endTime *int64) ([]tongo.Bits256, error) {
	return nil, nil
}
