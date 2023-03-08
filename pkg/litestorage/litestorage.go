package litestorage

import (
	"context"
	"fmt"
	"time"

	retry "github.com/avast/retry-go"
	"go.uber.org/zap"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/tlb"

	"github.com/tonkeeper/opentonapi/pkg/core"
)

type LiteStorage struct {
	client                  *liteapi.Client
	transactionsIndex       map[tongo.AccountID][]*core.Transaction
	transactionsIndexByHash map[tongo.Bits256]*core.Transaction
	blockCache              map[tongo.BlockIDExt]*tlb.Block
}

func (s *LiteStorage) GetAccount(ctx context.Context, address tongo.AccountID) (*core.Account, error) {
	var account tlb.Account
	err := retry.Do(func() error {
		state, err := s.client.GetAccountState(ctx, address)
		if err != nil {
			return err
		}
		account = state.Account
		return nil
	}, retry.Attempts(10), retry.Delay(10*time.Millisecond))

	if err != nil {
		return nil, err
	}
	return core.ConvertToAccount(address, account)
}

func NewLiteStorage(preloadAccounts []tongo.AccountID, log *zap.Logger) (*LiteStorage, error) {
	client, err := liteapi.NewClientWithDefaultMainnet()
	if err != nil {
		return nil, err
	}

	l := &LiteStorage{
		client:                  client,
		transactionsIndex:       make(map[tongo.AccountID][]*core.Transaction),
		transactionsIndexByHash: make(map[tongo.Bits256]*core.Transaction),
		blockCache:              make(map[tongo.BlockIDExt]*tlb.Block),
	}
	for _, a := range preloadAccounts {
		l.preloadAccount(a, log)
	}
	return l, nil
}

func (s *LiteStorage) preloadAccount(a tongo.AccountID, log *zap.Logger) error {
	ctx := context.Background()
	accountTxs, err := s.client.GetLastTransactions(ctx, a, 1000)
	if err != nil {
		return err
	}
	for _, tx := range accountTxs {
		t, err := core.ConvertTransaction(a.Workchain, tx)
		if err != nil {
			return err
		}
		s.transactionsIndexByHash[tongo.Bits256(tx.Hash())] = t
		s.transactionsIndex[a] = append(s.transactionsIndex[a], t)
	}

	return nil
}

func (s *LiteStorage) GetBlockHeader(ctx context.Context, id tongo.BlockID) (*core.BlockHeader, error) {
	return nil, fmt.Errorf("fuck you")
	blockID, _, err := s.client.LookupBlock(ctx, id, 1, nil, nil)
	if err != nil {
		return nil, err
	}
	block, err := s.client.GetBlock(ctx, blockID)
	if err != nil {
		return nil, err
	}
	s.blockCache[blockID] = &block
	header, err := core.ConvertToBlockHeader(blockID, &block)
	if err != nil {
		return nil, err
	}
	return header, nil
}

func (s *LiteStorage) GetTransaction(ctx context.Context, hash tongo.Bits256) (*core.Transaction, error) {
	tx, prs := s.transactionsIndexByHash[hash]
	if prs {
		return tx, nil
	}
	return nil, fmt.Errorf("not found tx %x", hash)
}

func (s *LiteStorage) searchTxInCache(a tongo.AccountID, lt uint64) *core.Transaction {
	return nil
}
