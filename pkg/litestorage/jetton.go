package litestorage

import (
	"context"
	"errors"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

func (s *LiteStorage) GetJettonWalletsByOwnerAddress(ctx context.Context, address tongo.AccountID) ([]core.JettonWallet, error) {
	wallets := []core.JettonWallet{}

	knownJettons := s.AddressBook.GetKnownJettons()
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
	if errors.Is(err, core.ErrEntityNotFound) {
		if !ok {
			return tongo.JettonMetadata{}, err
		}
		rawMeta = tongo.JettonMetadata{
			Name:  "Unknown",
			Image: "https://ton.ams3.digitaloceanspaces.com/token-placeholder-288.png",
		}
	} else if err != nil {
		return tongo.JettonMetadata{}, err
	}
	s.jettonMetaCache[master.ToRaw()] = rawMeta
	return rawMeta, nil
}

func rewriteIfNotEmpty(src, dest string) string {
	if dest != "" {
		return dest
	}
	return src
}
