package litestorage

import (
	"context"
	"fmt"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/tongo/config"
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
	AddressBook             *addressbook.Book
	transactionsIndex       map[tongo.AccountID][]*core.Transaction
	jettonMetaCache         map[string]core.JettonMetadata
	transactionsIndexByHash map[tongo.Bits256]*core.Transaction
	blockCache              map[tongo.BlockIDExt]*tlb.Block
}

type Options struct {
	preloadAccounts []tongo.AccountID
	addressBook     *addressbook.Book
	servers         []config.LiteServer
}

func WithPreloadAccounts(a []tongo.AccountID) Option {
	return func(o *Options) {
		o.preloadAccounts = a
	}
}

func WithKnownJettons(addressbook *addressbook.Book) Option {
	return func(o *Options) {
		o.addressBook = addressbook
	}
}

func WithLiteServers(servers []config.LiteServer) Option {
	return func(o *Options) {
		o.servers = servers
	}
}

type Option func(o *Options)

func NewLiteStorage(log *zap.Logger, opts ...Option) (*LiteStorage, error) {
	o := &Options{}
	for i := range opts {
		opts[i](o)
	}

	client, err := liteapi.NewClientWithDefaultMainnet()
	if err != nil {
		return nil, err
	}

	l := &LiteStorage{
		client:                  client,
		AddressBook:             o.addressBook,
		transactionsIndex:       make(map[tongo.AccountID][]*core.Transaction),
		jettonMetaCache:         make(map[string]core.JettonMetadata),
		transactionsIndexByHash: make(map[tongo.Bits256]*core.Transaction),
		blockCache:              make(map[tongo.BlockIDExt]*tlb.Block),
	}
	for _, a := range o.preloadAccounts {
		l.preloadAccount(a, log)
	}
	return l, nil
}

// GetAccountInfo returns human-friendly information about an account without low-level details.
func (s *LiteStorage) GetAccountInfo(ctx context.Context, id tongo.AccountID) (*core.AccountInfo, error) {
	account, err := s.GetRawAccount(ctx, id)
	if err != nil {
		return nil, err
	}
	return &core.AccountInfo{
		Account: *account,
	}, nil
}

// GetRawAccount returns low-level information about an account taken directly from the blockchain.
func (s *LiteStorage) GetRawAccount(ctx context.Context, address tongo.AccountID) (*core.Account, error) {
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

func (s *LiteStorage) GetBlockTransactions(ctx context.Context, id tongo.BlockID) ([]*core.Transaction, error) {
	blockID, _, err := s.client.LookupBlock(ctx, id, 1, nil, nil)
	if err != nil {
		return nil, err
	}
	block, err := s.client.GetBlock(ctx, blockID)
	if err != nil {
		return nil, err
	}
	return core.ExtractTransactions(blockID, &block)
}

func (s *LiteStorage) searchTxInCache(a tongo.AccountID, lt uint64) *core.Transaction {
	return nil
}
