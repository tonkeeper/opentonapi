package litestorage

import (
	"context"
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/tonkeeper/tongo/tlb"
	tongoWallet "github.com/tonkeeper/tongo/wallet"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
)

func (s *LiteStorage) GetSubscriptions(ctx context.Context, address tongo.AccountID) ([]core.Subscription, error) {
	return []core.Subscription{}, nil
}

func (s *LiteStorage) GetSeqno(ctx context.Context, account tongo.AccountID) (uint32, error) {
	return s.client.GetSeqno(ctx, account)
}

func (s *LiteStorage) GetAccountState(ctx context.Context, a tongo.AccountID) (tlb.ShardAccount, error) {
	return s.client.GetAccountState(ctx, a)
}

func (s *LiteStorage) SearchAccountsByPubKey(pubKey ed25519.PublicKey) ([]tongo.AccountID, error) {
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
		s.pubKeyByAccountID.Store(walletAddress, pubKey)
	}
	return walletAddresses, nil
}

func (s *LiteStorage) GetAccountDiff(ctx context.Context, account tongo.AccountID, startTime int64, endTime int64) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *LiteStorage) GetLatencyAndLastMasterchainSeqno(ctx context.Context) (int64, uint32, error) {
	blockHeader, err := s.LastMasterchainBlockHeader(ctx)
	if err != nil {
		return 0, 0, err
	}
	latency := time.Now().Unix() - int64(blockHeader.GenUtime)
	return latency, blockHeader.Seqno, nil
}
