package litestorage

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"github.com/tonkeeper/tongo/boc"
	"math/big"
	"time"

	"github.com/avast/retry-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/puzpuzpuz/xsync/v2"
	"github.com/sourcegraph/conc/iter"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/config"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/tep64"
	"github.com/tonkeeper/tongo/tlb"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/blockchain/indexer"
	"github.com/tonkeeper/opentonapi/pkg/core"
)

var storageTimeHistogramVec = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "litestorage_functions_time",
		Help:    "LiteStorage functions execution duration distribution in seconds",
		Buckets: []float64{0.001, 0.01, 0.05, 0.1, 1, 5, 10},
	},
	[]string{"method"},
)

// inMsgCreatedLT is used as a key to look up a transaction's hash based on in msg's account and created lt.
type inMsgCreatedLT struct {
	account tongo.AccountID
	lt      uint64
}

func extractInMsgCreatedLT(accountID tongo.AccountID, tx *tlb.Transaction) (inMsgCreatedLT, bool) {
	if tx.Msgs.InMsg.Exists && tx.Msgs.InMsg.Value.Value.Info.IntMsgInfo != nil {
		return inMsgCreatedLT{account: accountID, lt: tx.Msgs.InMsg.Value.Value.Info.IntMsgInfo.CreatedLt}, true
	}
	return inMsgCreatedLT{}, false
}

type LiteStorage struct {
	logger                  *zap.Logger
	client                  *liteapi.Client
	jettonMetaCache         *xsync.MapOf[string, tep64.Metadata]
	transactionsIndexByHash *xsync.MapOf[tongo.Bits256, *core.Transaction]
	transactionsByInMsgLT   *xsync.MapOf[inMsgCreatedLT, tongo.Bits256]
	blockCache              *xsync.MapOf[tongo.BlockIDExt, *tlb.Block]
	accountInterfacesCache  *xsync.MapOf[tongo.AccountID, []abi.ContractInterface]
	knownAccounts           map[string][]tongo.AccountID
	// maxGoroutines specifies a number of goroutines used to perform some time-consuming operations.
	maxGoroutines int
	// trackingAccounts is a list of accounts we track. Defined with ACCOUNTS env variable.
	trackingAccounts map[tongo.AccountID]struct{}
}

type Options struct {
	preloadAccounts []tongo.AccountID
	preloadBlocks   []tongo.BlockID
	servers         []config.LiteServer
	tfPools         []tongo.AccountID
	jettons         []tongo.AccountID
	// blockCh is used to receive new blocks in the blockchain, if set.
	blockCh <-chan indexer.IDandBlock
}

func WithPreloadAccounts(a []tongo.AccountID) Option {
	return func(o *Options) {
		o.preloadAccounts = a
	}
}

func WithPreloadBlocks(ids []tongo.BlockID) Option {
	return func(o *Options) {
		o.preloadBlocks = ids
	}
}

func WithKnownJettons(a []tongo.AccountID) Option {
	return func(o *Options) {
		o.jettons = a
	}
}

func WithLiteServers(servers []config.LiteServer) Option {
	return func(o *Options) {
		o.servers = servers
	}
}

func WithTFPools(pools []tongo.AccountID) Option {
	return func(o *Options) {
		o.tfPools = pools
	}
}

// WithBlockChannel configures a channel to receive notifications about new blocks in the blockchain.
func WithBlockChannel(ch <-chan indexer.IDandBlock) Option {
	return func(o *Options) {
		o.blockCh = ch
	}
}

type Option func(o *Options)

func NewLiteStorage(log *zap.Logger, opts ...Option) (*LiteStorage, error) {
	o := &Options{}
	for i := range opts {
		opts[i](o)
	}
	var err error
	var client *liteapi.Client
	if len(o.servers) == 0 {
		log.Warn("USING PUBLIC CONFIG! BE CAREFUL!")
		client, err = liteapi.NewClientWithDefaultMainnet()
	} else {
		client, err = liteapi.NewClient(liteapi.WithLiteServers(o.servers))
	}
	if err != nil {
		return nil, err
	}

	storage := &LiteStorage{
		logger: log,
		// TODO: introduce an env variable to configure this number
		maxGoroutines: 5,
		client:        client,
		// read-only data
		knownAccounts:    make(map[string][]tongo.AccountID),
		trackingAccounts: map[tongo.AccountID]struct{}{},
		// data for concurrent access
		// TODO: implement expiration logic for the caches below.
		jettonMetaCache:         xsync.NewMapOf[tep64.Metadata](),
		transactionsIndexByHash: xsync.NewTypedMapOf[tongo.Bits256, *core.Transaction](hashBits256),
		transactionsByInMsgLT:   xsync.NewTypedMapOf[inMsgCreatedLT, tongo.Bits256](hashInMsgCreatedLT),
		blockCache:              xsync.NewTypedMapOf[tongo.BlockIDExt, *tlb.Block](hashBlockIDExt),
		accountInterfacesCache:  xsync.NewTypedMapOf[tongo.AccountID, []abi.ContractInterface](hashAccountID),
	}
	storage.knownAccounts["tf_pools"] = o.tfPools
	storage.knownAccounts["jettons"] = o.jettons

	for _, a := range o.preloadAccounts {
		storage.trackingAccounts[a] = struct{}{}
	}

	blockIterator := iter.Iterator[tongo.BlockID]{MaxGoroutines: storage.maxGoroutines}
	blockIterator.ForEach(o.preloadBlocks, func(id *tongo.BlockID) {
		if err := storage.preloadBlock(*id); err != nil {
			log.Error("failed to preload block",
				zap.String("blockID", id.String()),
				zap.Error(err))
		}
	})
	iterator := iter.Iterator[tongo.AccountID]{MaxGoroutines: storage.maxGoroutines}
	iterator.ForEach(o.preloadAccounts, func(accountID *tongo.AccountID) {
		if err := storage.preloadAccount(*accountID); err != nil {
			log.Error("failed to preload account",
				zap.String("accountID", accountID.String()),
				zap.Error(err))
		}
	})
	go storage.run(o.blockCh)
	return storage, nil
}

func (s *LiteStorage) run(ch <-chan indexer.IDandBlock) {
	if ch == nil {
		return
	}
	for block := range ch {
		for _, tx := range block.Block.AllTransactions() {
			accountID := *tongo.NewAccountId(block.ID.Workchain, tx.AccountAddr)
			if _, ok := s.trackingAccounts[accountID]; ok {
				hash := tongo.Bits256(tx.Hash())
				transaction, err := core.ConvertTransaction(accountID.Workchain, tongo.Transaction{Transaction: *tx, BlockID: block.ID})
				if err != nil {
					s.logger.Error("failed to process tx",
						zap.String("tx-hash", hash.Hex()),
						zap.Error(err))
					continue
				}
				s.transactionsIndexByHash.Store(hash, transaction)
				if createLT, ok := extractInMsgCreatedLT(accountID, tx); ok {
					s.transactionsByInMsgLT.Store(createLT, hash)
				}
			}
		}
	}
}

// GetRawAccount returns low-level information about an account taken directly from the blockchain.
func (s *LiteStorage) GetRawAccount(ctx context.Context, address tongo.AccountID) (*core.Account, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_raw_account").Observe(v)
	}))
	defer timer.ObserveDuration()
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

// GetRawAccounts returns low-level information about several accounts taken directly from the blockchain.
func (s *LiteStorage) GetRawAccounts(ctx context.Context, ids []tongo.AccountID) ([]*core.Account, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_raw_accounts").Observe(v)
	}))
	defer timer.ObserveDuration()
	var accounts []*core.Account
	for _, address := range ids {
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
		acc, err := core.ConvertToAccount(address, account)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, acc)
	}
	return accounts, nil
}

func (s *LiteStorage) preloadAccount(a tongo.AccountID) error {
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
		hash := tongo.Bits256(tx.Hash())
		s.transactionsIndexByHash.Store(hash, t)
		if createLT, ok := extractInMsgCreatedLT(a, &tx.Transaction); ok {
			s.transactionsByInMsgLT.Store(createLT, hash)
		}
	}
	return nil
}

func (s *LiteStorage) preloadBlock(id tongo.BlockID) error {
	ctx := context.Background()
	extID, _, err := s.client.LookupBlock(ctx, id, 1, nil, nil)
	if err != nil {
		return err
	}
	block, err := s.client.GetBlock(ctx, extID)
	if err != nil {
		return err
	}
	s.blockCache.Store(extID, &block)
	for _, tx := range block.AllTransactions() {
		accountID := tongo.AccountID{
			Workchain: extID.Workchain,
			Address:   tx.AccountAddr,
		}
		t, err := core.ConvertTransaction(extID.Workchain, tongo.Transaction{Transaction: *tx, BlockID: extID})
		if err != nil {
			return err
		}
		hash := tongo.Bits256(tx.Hash())
		s.transactionsIndexByHash.Store(hash, t)
		if createLT, ok := extractInMsgCreatedLT(accountID, tx); ok {
			s.transactionsByInMsgLT.Store(createLT, hash)
		}
	}
	return nil
}

func (s *LiteStorage) GetBlockHeader(ctx context.Context, id tongo.BlockID) (*core.BlockHeader, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_block_header").Observe(v)
	}))
	defer timer.ObserveDuration()
	blockID, _, err := s.client.LookupBlock(ctx, id, 1, nil, nil)
	if err != nil {
		return nil, err
	}
	block, err := s.client.GetBlock(ctx, blockID)
	if err != nil {
		return nil, err
	}

	s.blockCache.Store(blockID, &block)
	header, err := core.ConvertToBlockHeader(blockID, &block)
	if err != nil {
		return nil, err
	}
	return header, nil
}

func (s *LiteStorage) LastMasterchainBlockHeader(ctx context.Context) (*core.BlockHeader, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_masterchain").Observe(v)
	}))
	defer timer.ObserveDuration()
	info, err := s.client.GetMasterchainInfo(ctx)
	if err != nil {
		return nil, err
	}
	return s.GetBlockHeader(ctx, info.Last.ToBlockIdExt().BlockID)
}

func (s *LiteStorage) GetTransaction(ctx context.Context, hash tongo.Bits256) (*core.Transaction, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_transaction").Observe(v)
	}))
	defer timer.ObserveDuration()
	tx, prs := s.transactionsIndexByHash.Load(hash)
	if prs {
		return tx, nil
	}
	return nil, fmt.Errorf("not found tx %x", hash)
}

func (s *LiteStorage) SearchTransactionByMessageHash(ctx context.Context, hash tongo.Bits256) (*tongo.Bits256, error) {
	//var txHash tongo.Bits256
	//found := false
	//s.transactionsIndexByHash.Range(func(key tongo.Bits256, value *core.Transaction) bool {
	//	if value.InMsg != nil && value.InMsg.Hash {
	//		txHash = key
	//		found =true
	//		return false
	//	}
	//	return true
	//})
	//if found {
	//	return &txHash, nil
	//}
	return nil, core.ErrEntityNotFound
}

func (s *LiteStorage) GetBlockTransactions(ctx context.Context, id tongo.BlockID) ([]*core.Transaction, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_block_transactions").Observe(v)
	}))
	defer timer.ObserveDuration()
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
	hash, ok := s.transactionsByInMsgLT.Load(inMsgCreatedLT{account: a, lt: lt})
	if !ok {
		return nil
	}
	tx, ok := s.transactionsIndexByHash.Load(hash)
	if !ok {
		return nil
	}
	return tx
}

func (s *LiteStorage) GetStorageProviders(ctx context.Context) ([]core.StorageProvider, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_storage_providers").Observe(v)
	}))
	defer timer.ObserveDuration()

	return nil, errors.New("not implemented")
}

func (s *LiteStorage) RunSmcMethod(ctx context.Context, id tongo.AccountID, method string, stack tlb.VmStack) (uint32, tlb.VmStack, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("run_smc_method").Observe(v)
	}))
	defer timer.ObserveDuration()
	return s.client.RunSmcMethod(ctx, id, method, stack)
}

func (s *LiteStorage) RunSmcMethodByID(ctx context.Context, id tongo.AccountID, method int, stack tlb.VmStack) (uint32, tlb.VmStack, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("run_smc_method_by_id").Observe(v)
	}))
	defer timer.ObserveDuration()
	return s.client.RunSmcMethodByID(ctx, id, method, stack)
}

func (s *LiteStorage) GetAccountTransactions(ctx context.Context, id tongo.AccountID, limit int, beforeLt, afterLt uint64) ([]*core.Transaction, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_account_transactions").Observe(v)
	}))
	defer timer.ObserveDuration()
	txs, err := s.client.GetLastTransactions(ctx, id, limit) //todo: custom with beforeLt and afterLt
	if err != nil {
		return nil, err
	}
	result := make([]*core.Transaction, len(txs))
	for i := range txs {
		tx, err := core.ConvertTransaction(id.Workchain, txs[i])
		if err != nil {
			return nil, err
		}
		result[i] = tx
	}
	return result, nil
}

func (s *LiteStorage) FindAllDomainsResolvedToAddress(ctx context.Context, a tongo.AccountID, collections map[tongo.AccountID]string) ([]string, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("find_all_domains_resolved_to_address").Observe(v)
	}))
	defer timer.ObserveDuration()
	return nil, nil
}

func (s *LiteStorage) GetWalletPubKey(ctx context.Context, address tongo.AccountID) (ed25519.PublicKey, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_wallet_by_pubkey").Observe(v)
	}))
	defer timer.ObserveDuration()
	_, result, err := abi.GetPublicKey(ctx, s.client, address)
	if err != nil {
		return nil, err
	}
	if r, ok := result.(abi.GetPublicKeyResult); ok {
		i := big.Int(r.PublicKey)
		b := i.Bytes()
		if len(b) < 24 || len(b) > 32 {
			return nil, fmt.Errorf("invalid publock key")
		}
		return append(make([]byte, 32-len(b)), b...), nil
	}
	return nil, fmt.Errorf("can't get publick key")
}

func (s *LiteStorage) ReindexAccount(ctx context.Context, accountID tongo.AccountID) error {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("reindex_account").Observe(v)
	}))
	defer timer.ObserveDuration()
	return nil
}

func (s *LiteStorage) GetDnsExpiring(ctx context.Context, id tongo.AccountID, period *int) ([]core.DnsExpiring, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_dns_expiring").Observe(v)
	}))
	defer timer.ObserveDuration()
	return nil, nil
}

func (s *LiteStorage) GetLibraries(ctx context.Context, libraries []tongo.Bits256) (map[tongo.Bits256]*boc.Cell, error) {
	return s.client.GetLibraries(ctx, libraries)
}
