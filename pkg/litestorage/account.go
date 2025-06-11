package litestorage

import (
	"context"
	"errors"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/ton"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
)

func (s *LiteStorage) GetSubscriptionsV2(ctx context.Context, address tongo.AccountID) ([]core.SubscriptionV2, error) {
	return []core.SubscriptionV2{}, nil
}

func (s *LiteStorage) GetSubscriptionsV1(ctx context.Context, address tongo.AccountID) ([]core.SubscriptionV1, error) {
	return []core.SubscriptionV1{}, nil
}

func (s *LiteStorage) GetSeqno(ctx context.Context, account tongo.AccountID) (uint32, error) {
	return s.client.GetSeqno(ctx, account)
}

func (s *LiteStorage) GetAccountState(ctx context.Context, a tongo.AccountID) (tlb.ShardAccount, error) {
	return s.client.GetAccountState(ctx, a)
}

func (s *LiteStorage) AccountStatusAndInterfaces(addr tongo.AccountID) (tlb.AccountStatus, []abi.ContractInterface, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	account, err := s.GetRawAccount(ctx, addr) //todo: get only 2 fields
	if errors.Is(err, core.ErrEntityNotFound) {
		return tlb.AccountNone, nil, nil
	}
	return account.Status, account.Interfaces, err
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

func (s *LiteStorage) GetAccountsStats(ctx context.Context, accounts []ton.AccountID) ([]core.AccountStat, error) {
	return nil, nil
}

func (s *LiteStorage) GetAccountPlugins(ctx context.Context, accountID ton.AccountID, walletVersion abi.ContractInterface) ([]core.Plugin, error) {
	return nil, nil
}
