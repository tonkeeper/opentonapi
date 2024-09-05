package litestorage

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shopspring/decimal"
	"github.com/sourcegraph/conc/iter"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/ton"
)

func (s *LiteStorage) GetJettonWalletsByOwnerAddress(ctx context.Context, address ton.AccountID, jetton *ton.AccountID, mintless bool) ([]core.JettonWallet, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_jetton_wallets_by_owner").Observe(v)
	}))
	defer timer.ObserveDuration()
	jettons := s.knownAccounts["jettons"]
	mapper := iter.Mapper[tongo.AccountID, *core.JettonWallet]{
		MaxGoroutines: s.maxGoroutines,
	}
	wallets, err := mapper.MapErr(jettons, func(jettonMaster *tongo.AccountID) (*core.JettonWallet, error) {
		_, result, err := abi.GetWalletAddress(ctx, s.executor, *jettonMaster, address.ToMsgAddress())
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
		_, result, err = abi.GetWalletData(ctx, s.executor, *walletAddress)
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
		wallet := core.JettonWallet{
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
	var results []core.JettonWallet
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

func (s *LiteStorage) GetJettonMasterData(ctx context.Context, master tongo.AccountID) (core.JettonMaster, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_jetton_master_data").Observe(v)
	}))
	defer timer.ObserveDuration()
	_, value, err := abi.GetJettonData(ctx, s.executor, master)
	if err != nil {
		return core.JettonMaster{}, err
	}
	r, ok := value.(abi.GetJettonDataResult)
	if !ok {
		return core.JettonMaster{}, fmt.Errorf("invalid jetton data result")
	}
	jettonMaster := core.JettonMaster{
		Address:     master,
		TotalSupply: big.Int(r.TotalSupply),
		Mintable:    r.Mintable,
	}
	jettonMaster.Admin, _ = tongo.AccountIDFromTlb(r.AdminAddress)
	return jettonMaster, nil
}

func (s *LiteStorage) GetAccountJettonsHistory(ctx context.Context, address tongo.AccountID, limit int, beforeLT, startTime, endTime *int64) ([]tongo.Bits256, error) {
	return nil, nil
}

func (s *LiteStorage) GetAccountJettonHistoryByID(ctx context.Context, address, jettonMaster tongo.AccountID, limit int, beforeLT, startTime, endTime *int64) ([]tongo.Bits256, error) {
	return nil, nil
}

func (s *LiteStorage) JettonMastersForWallets(ctx context.Context, wallets []tongo.AccountID) (map[tongo.AccountID]tongo.AccountID, error) {
	masters := make(map[tongo.AccountID]tongo.AccountID)
	for _, wallet := range wallets {
		_, value, err := abi.GetWalletData(ctx, s.executor, wallet)
		if err != nil {
			return nil, err
		}
		data, ok := value.(abi.GetWalletDataResult)
		if !ok {
			continue
		}
		master, err := tongo.AccountIDFromTlb(data.Jetton)
		if err != nil {
			return nil, err
		}
		if master != nil {
			masters[wallet] = *master
		}
	}
	return masters, nil
}

func (s *LiteStorage) GetJettonMasters(ctx context.Context, limit, offset int) ([]core.JettonMaster, error) {
	// TODO: implement
	return nil, nil
}

func (s *LiteStorage) GetJettonsHoldersCount(ctx context.Context, accountIDs []tongo.AccountID) (map[tongo.AccountID]int32, error) {
	return map[tongo.AccountID]int32{}, nil
}

func (s *LiteStorage) GetJettonHolders(ctx context.Context, jettonMaster tongo.AccountID, limit, offset int) ([]core.JettonHolder, error) {
	return []core.JettonHolder{}, nil
}
