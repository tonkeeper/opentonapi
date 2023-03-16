package litestorage

import (
	"context"
	"errors"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

func (s *LiteStorage) GetJettonWalletsByOwnerAddress(ctx context.Context, address tongo.AccountID, knownJettons []addressbook.KnownJetton) ([]core.JettonWallet, error) {
	wallets := []core.JettonWallet{}

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
		wallets = append(wallets, result.(core.JettonWallet))
	}

	return wallets, nil
}

func (s *LiteStorage) GetJettonMasterMetadata(ctx context.Context, master tongo.AccountID, book *addressbook.Book) (core.JettonMetadata, error) {
	meta, ok := s.jettonMetaCache[master.ToRaw()]
	if ok {
		return meta, nil
	}
	info, ok := book.GetJettonInfoByAddress(master.ToRaw())
	rawMeta, err := s.client.GetJettonData(ctx, master)
	if errors.Is(err, core.ErrEntityNotFound) {
		if !ok {
			return core.JettonMetadata{}, err
		}
		rawMeta = tongo.JettonMetadata{
			Name:  "Unknown",
			Image: "https://ton.ams3.digitaloceanspaces.com/token-placeholder-288.png",
		}
	} else if err != nil {
		return core.JettonMetadata{}, err
	}
	res := core.ConvertJettonMeta(rawMeta)
	res.Address = master
	res.Name = rewriteIfNotEmpty(res.Name, info.Name)
	res.Description = rewriteIfNotEmpty(res.Description, info.Description)
	res.Image = rewriteIfNotEmpty(res.Image, info.Image)
	res.Symbol = rewriteIfNotEmpty(res.Symbol, info.Symbol)
	res.Verification = info.Verification
	s.jettonMetaCache[master.ToRaw()] = res // TODO: is it okay, if cache not expire?
	return res, nil
}

func rewriteIfNotEmpty(src, dest string) string {
	if dest != "" {
		return dest
	}
	return src
}
