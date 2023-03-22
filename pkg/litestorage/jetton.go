package litestorage

import (
	"context"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

func (s *LiteStorage) GetJettonWalletsByOwnerAddress(ctx context.Context, address tongo.AccountID) ([]core.JettonWallet, error) {
	wallets := []core.JettonWallet{}

	knownJettons := s.addressBook.GetKnownJettons()
	for _, jetton := range knownJettons {
		jettonAddress, _ := tongo.ParseAccountID(jetton.Address)
		_, result, err := abi.GetWalletAddress(ctx, s.client, jettonAddress, address.ToMsgAddress())
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
		if jettonWallet.Address != jettonAddress {
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
