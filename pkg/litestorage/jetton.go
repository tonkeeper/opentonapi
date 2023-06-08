package litestorage

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/tonkeeper/opentonapi/pkg/core/jetton"

	"github.com/shopspring/decimal"
	"github.com/sourcegraph/conc/iter"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/liteapi"
)

func (s *LiteStorage) GetJettonWalletsByOwnerAddress(ctx context.Context, address tongo.AccountID) ([]jetton.Wallet, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_jetton_wallets_by_owner").Observe(v)
	}))
	defer timer.ObserveDuration()
	jettons := s.knownAccounts["jettons"]
	mapper := iter.Mapper[tongo.AccountID, *jetton.Wallet]{
		MaxGoroutines: s.maxGoroutines,
	}
	wallets, err := mapper.MapErr(jettons, func(jettonMaster *tongo.AccountID) (*jetton.Wallet, error) {
		_, result, err := abi.GetWalletAddress(ctx, s.client, *jettonMaster, address.ToMsgAddress())
		if err != nil {
			return nil, err
		}
		addressResult := result.(abi.GetWalletAddressResult)
		walletAddress, err := tongo.AccountIDFromTlb(addressResult.JettonWalletAddress)
		if err != nil {
			return nil, err
		}
		if walletAddress == nil {
			return nil, nil
		}
		_, result, err = abi.GetWalletData(ctx, s.client, *walletAddress)
		if err != nil && errors.Is(err, liteapi.ErrAccountNotFound) {
			// our account doesn't have a corresponding wallet account, ignore this error.
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		jettonWallet := result.(abi.GetWalletDataResult)
		jettonAddress, err := tongo.AccountIDFromTlb(addressResult.JettonWalletAddress)
		if err != nil {
			return nil, err
		}
		if jettonAddress == nil {
			return nil, nil
		}
		if jettonAddress == nil || *jettonAddress != *walletAddress {
			return nil, nil
		}
		balance := big.Int(jettonWallet.Balance)
		wallet := jetton.Wallet{
			Address:       *walletAddress,
			Balance:       decimal.NewFromBigInt(&balance, 0),
			OwnerAddress:  &address,
			JettonAddress: *jettonMaster,
		}
		return &wallet, nil
	})
	if err != nil {
		return nil, err
	}
	var results []jetton.Wallet
	for _, wallet := range wallets {
		if wallet != nil {
			results = append(results, *wallet)
		}
	}
	return results, nil
}

func (s *LiteStorage) GetJettonMasterMetadata(ctx context.Context, master tongo.AccountID) (tongo.JettonMetadata, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_jetton_master_metadata").Observe(v)
	}))
	defer timer.ObserveDuration()
	meta, ok := s.jettonMetaCache.Load(master.ToRaw())
	if ok {
		return meta, nil
	}
	rawMeta, err := s.client.GetJettonData(ctx, master)
	if err != nil {
		return tongo.JettonMetadata{}, err
	}
	s.jettonMetaCache.Store(master.ToRaw(), rawMeta)
	return rawMeta, nil
}

func (s *LiteStorage) GetJettonMasterData(ctx context.Context, master tongo.AccountID) (abi.GetJettonDataResult, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_jetton_master_data").Observe(v)
	}))
	defer timer.ObserveDuration()
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
