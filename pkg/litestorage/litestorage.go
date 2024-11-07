package litestorage

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/puzpuzpuz/xsync/v2"
	"github.com/sourcegraph/conc/iter"
	"github.com/tonkeeper/tonapi-go"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/tep64"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"

	"github.com/arnac-io/opentonapi/pkg/blockchain/indexer"
	"github.com/arnac-io/opentonapi/pkg/cache"
	"github.com/arnac-io/opentonapi/pkg/core"
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

func (m inMsgCreatedLT) String() string {
	return fmt.Sprintf("%s_%d", m.account.String(), m.lt)
}

func extractInMsgCreatedLT(accountID tongo.AccountID, tx core.Transaction) (inMsgCreatedLT, bool) {
	if tx.InMsg != nil {
		return inMsgCreatedLT{account: accountID, lt: tx.InMsg.CreatedLt}, true
	}
	return inMsgCreatedLT{}, false
}

type LiteStorage struct {
	logger       *zap.Logger
	client       *liteapi.Client
	tonapiClient *tonapi.Client

	executor                abi.Executor
	jettonMetaCache         *xsync.MapOf[string, tep64.Metadata]
	transactionsIndexByHash ICache[core.Transaction]
	transactionsByInMsg     ICache[string]
	transactionsByOutMsg    ICache[string]
	cacheExpiration         time.Duration
	blockCache              *xsync.MapOf[tongo.BlockIDExt, *tlb.Block]
	accountInterfacesCache  *xsync.MapOf[tongo.AccountID, []abi.ContractInterface]
	// tvmLibraryCache contains public tvm libraries.
	// As a library is immutable, it's ok to cache it.
	tvmLibraryCache cache.Cache[string, boc.Cell]
	knownAccounts   map[string][]tongo.AccountID
	// maxGoroutines specifies a number of goroutines used to perform some time-consuming operations.
	maxGoroutines int
	// trackingAccounts is a list of accounts we track. Defined with ACCOUNTS env variable.
	trackingAccounts  map[tongo.AccountID]struct{}
	trackAllAccounts  bool
	pubKeyByAccountID *xsync.MapOf[tongo.AccountID, ed25519.PublicKey]
	configCache       cache.Cache[int, ton.BlockchainConfig]

	stopCh chan struct{}
	// mu protects trimmedConfigBase64.
	mu sync.RWMutex
	// trimmedConfigBase64 is a blockchain config but with a limited set of keys.
	// it's performance optimization.
	// tmv and txEmulator work much faster with a smaller config.
	trimmedConfigBase64 string
}

type Options struct {
	preloadAccounts  []tongo.AccountID
	trackAllAccounts bool
	preloadBlocks    []tongo.BlockID
	tfPools          []tongo.AccountID
	jettons          []tongo.AccountID
	executor         abi.Executor
	// blockCh is used to receive new blocks in the blockchain, if set.
	blockCh <-chan indexer.IDandBlock

	transactionsIndexByHash ICache[core.Transaction]
	transactionsByInMsg     ICache[string]
	transactionsByOutMsg    ICache[string]
	cacheExpiration         time.Duration
	tonApiClient            *tonapi.Client
}

func WithPreloadAccounts(a []tongo.AccountID) Option {
	return func(o *Options) {
		o.preloadAccounts = a
	}
}

func WithTrackAllAccounts() Option {
	return func(o *Options) {
		o.trackAllAccounts = true
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

func WithTransactionsIndexByHash(cache ICache[core.Transaction]) Option {
	return func(o *Options) {
		o.transactionsIndexByHash = cache
	}
}

func WithTransactionsByInMsg(cache ICache[string]) Option {
	return func(o *Options) {
		o.transactionsByInMsg = cache
	}
}

func WithTransactionsByOutMsg(cache ICache[string]) Option {
	return func(o *Options) {
		o.transactionsByOutMsg = cache
	}
}

func WithCacheExpiration(d time.Duration) Option {
	return func(o *Options) {
		o.cacheExpiration = d
	}
}

func WithTonApiClient(c *tonapi.Client) Option {
	return func(o *Options) {
		o.tonApiClient = c
	}
}

type Option func(o *Options)

func NewLiteStorage(log *zap.Logger, cli *liteapi.Client, opts ...Option) (*LiteStorage, error) {
	o := &Options{}
	for i := range opts {
		opts[i](o)
	}
	if o.executor == nil {
		o.executor = cli
	}
	storage := &LiteStorage{
		logger: log,
		// TODO: introduce an env variable to configure this number
		maxGoroutines: 5,
		client:        cli,
		tonapiClient:  o.tonApiClient,
		executor:      o.executor,
		stopCh:        make(chan struct{}),
		// read-only data
		knownAccounts:    make(map[string][]tongo.AccountID),
		trackingAccounts: map[tongo.AccountID]struct{}{},
		trackAllAccounts: o.trackAllAccounts,
		// data for concurrent access
		// TODO: implement expiration logic for the caches below.
		jettonMetaCache:         xsync.NewMapOf[tep64.Metadata](),
		transactionsIndexByHash: o.transactionsIndexByHash,
		transactionsByInMsg:     o.transactionsByInMsg,
		transactionsByOutMsg:    o.transactionsByOutMsg,
		cacheExpiration:         o.cacheExpiration,
		blockCache:              xsync.NewTypedMapOf[tongo.BlockIDExt, *tlb.Block](hashBlockIDExt),
		accountInterfacesCache:  xsync.NewTypedMapOf[tongo.AccountID, []abi.ContractInterface](hashAccountID),
		pubKeyByAccountID:       xsync.NewTypedMapOf[tongo.AccountID, ed25519.PublicKey](hashAccountID),
		tvmLibraryCache:         cache.NewLRUCache[string, boc.Cell](10000, "tvm_libraries"),
		configCache:             cache.NewLRUCache[int, ton.BlockchainConfig](4, "config"),
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

func (s *LiteStorage) SetExecutor(e abi.Executor) {
	s.executor = e
}

// Shutdown stops all background goroutines.
func (s *LiteStorage) Shutdown() {
	s.stopCh <- struct{}{}
}

func getKeyFromMessage(msg core.Message) string {
	// Note, we use as a key a combination of all attributes (hash, createdLt, source, destination)
	// Previously, only source\destination + createdLt were used, but that is not unique enough.
	// Hash is unique enough, but can't be used on it's own, since in the settings of localenv hash is something empty
	// (This is due to a limitation of using the opentonapi open source version on the node).
	key := fmt.Sprintf("%s:%d", msg.Hash.Hex(), msg.CreatedLt)
	if msg.Source != nil {
		key = fmt.Sprintf("%s:%s", key, msg.Source.String())
	}
	if msg.Destination != nil {
		key = fmt.Sprintf("%s:%s", key, msg.Destination.String())
	}
	return key
}

func (s *LiteStorage) ParseBlock(ctx context.Context, block indexer.IDandBlock) {
	transactionsIndexByHash := map[string]core.Transaction{}
	transactionsByInMsg := map[string]string{}
	transactionsByOutMsg := map[string]string{}
	for _, tx := range block.Transactions {
		accountID := ton.MustParseAccountID(tx.Account.Address)
		_, trackSpecificAccount := s.trackingAccounts[accountID]
		if trackSpecificAccount || s.trackAllAccounts {
			transaction, err := OasTransactionToCoreTransaction(tx)
			if err != nil {
				s.logger.Error("failed to process tx",
					zap.String("tx-hash", tx.Hash),
					zap.Error(err))
				continue
			}
			hash := tongo.Bits256(transaction.Hash)
			transactionsIndexByHash[hash.Hex()] = transaction

			// We save in cache for each transaction:
			// 1. A map from the incoming message to the transaction hash (used when looking a transaction children)
			// 2. A map from each outgoing message to the transaction hash (used when looking a transaction parent)
			if transaction.InMsg != nil {
				transactionsByInMsg[getKeyFromMessage(*transaction.InMsg)] = hash.Hex()
			}

			for _, outMsg := range transaction.OutMsgs {
				transactionsByOutMsg[getKeyFromMessage(outMsg)] = hash.Hex()
			}
		}
	}
	s.logger.Info(fmt.Sprintf("Parsed block %s that contained %d transactions", block.ID.BlockID.String(), len(block.Transactions)))
	s.logger.Info(fmt.Sprintf("transactionsIndexByHash: %+v", transactionsIndexByHash))
	s.logger.Info(fmt.Sprintf("transactionsByInMsg: %s", transactionsByInMsg))
	s.logger.Info(fmt.Sprintf("transactionsByOutMsg: %s", transactionsByOutMsg))
	s.transactionsIndexByHash.SetMany(ctx, transactionsIndexByHash, s.cacheExpiration)
	s.transactionsByInMsg.SetMany(ctx, transactionsByInMsg, s.cacheExpiration)
	s.transactionsByOutMsg.SetMany(ctx, transactionsByOutMsg, s.cacheExpiration)
}

func (s *LiteStorage) run(ch <-chan indexer.IDandBlock) {
	if ch == nil {
		return
	}
	for block := range ch {
		s.ParseBlock(context.Background(), block)
	}
}

func (s *LiteStorage) GetContract(ctx context.Context, id tongo.AccountID) (*core.Contract, error) {
	account, err := s.GetRawAccount(ctx, id)
	if err != nil {
		return nil, err
	}
	return &core.Contract{
		Balance:           account.TonBalance,
		Status:            account.Status,
		Code:              account.Code,
		Data:              account.Data,
		Libraries:         account.Libraries,
		LastTransactionLt: account.LastTransactionLt,
	}, nil
}

// GetRawAccount returns low-level information about an account taken directly from the blockchain.
func (s *LiteStorage) GetRawAccount(ctx context.Context, address tongo.AccountID) (*core.Account, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_raw_account").Observe(v)
	}))
	defer timer.ObserveDuration()

	rawAccount, err := s.tonapiClient.GetBlockchainRawAccount(ctx, tonapi.GetBlockchainRawAccountParams{AccountID: address.ToRaw()})
	if err != nil {
		return nil, err
	}
	coreAccount, err := oasAccountToCoreAccount(*rawAccount)
	if err != nil {
		return nil, err
	}
	return &coreAccount, nil
}

// GetRawAccounts returns low-level information about several accounts taken directly from the blockchain.
func (s *LiteStorage) GetRawAccounts(ctx context.Context, ids []tongo.AccountID) ([]*core.Account, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_raw_accounts").Observe(v)
	}))
	defer timer.ObserveDuration()
	var accounts []*core.Account
	for _, address := range ids {
		var account tlb.ShardAccount
		err := retry.Do(func() error {
			state, err := s.client.GetAccountState(ctx, address)
			if err != nil {
				return err
			}
			account = state
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
	accountTxs, err := s.client.GetLastTransactions(ctx, a, 2000)
	if err != nil {
		return err
	}
	for _, tx := range accountTxs {
		inspector := abi.NewContractInspector(abi.InspectWithLibraryResolver(s))
		account, err := s.GetRawAccount(ctx, a)
		if err != nil {
			return err
		}
		cd, err := inspector.InspectContract(ctx, account.Code, s.executor, a)
		t, err := core.ConvertTransaction(a.Workchain, tx, cd)
		if err != nil {
			return err
		}
		hash := tongo.Bits256(tx.Hash())
		s.transactionsIndexByHash.Set(context.Background(), hash.Hex(), *t, s.cacheExpiration)
		// if createLT, ok := extractInMsgCreatedLT(a, &tx.Transaction); ok {
		// 	s.transactionsByInMsgLT.Set(context.Background(), createLT.String(), hash.Hex(), s.cacheExpiration)
		// }
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
		// accountID := tongo.AccountID{
		// 	Workchain: extID.Workchain,
		// 	Address:   tx.AccountAddr,
		// }
		t, err := core.ConvertTransaction(extID.Workchain, tongo.Transaction{Transaction: *tx, BlockID: extID}, nil)
		if err != nil {
			return err
		}
		hash := tongo.Bits256(tx.Hash())
		s.transactionsIndexByHash.Set(ctx, hash.Hex(), *t, s.cacheExpiration)
		// if createLT, ok := extractInMsgCreatedLT(accountID, tx); ok {
		// 	s.transactionsByInMsgLT.Set(ctx, createLT.String(), hash.Hex(), s.cacheExpiration)
		// }
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

func (s *LiteStorage) GetBlockShards(ctx context.Context, id tongo.BlockID) ([]ton.BlockID, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_block_shards").Observe(v)
	}))
	defer timer.ObserveDuration()
	blockID, _, err := s.client.LookupBlock(ctx, id, 1, nil, nil)
	if err != nil {
		return nil, err
	}
	shards, err := s.client.GetAllShardsInfo(ctx, blockID)
	if err != nil {
		return nil, err
	}
	res := make([]ton.BlockID, len(shards))
	for i, s := range shards {
		res[i] = s.BlockID
	}
	sort.Slice(shards, func(i, j int) bool {
		return shards[i].Shard < shards[j].Shard
	})
	return res, nil
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
	tx, err := s.getTxFromCacheByHash(hash.Hex())
	if err != nil {
		return nil, err
	}
	return tx, nil
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

func (s *LiteStorage) getTxFromCacheByHash(hash string) (*core.Transaction, error) {
	tx, err := s.transactionsIndexByHash.Get(context.Background(), hash)
	if err != nil {
		return nil, core.ErrEntityNotFound
	}

	// Message contain decoded message body which are of type Any
	// This means that after we saved them to the cache, and reread them, we get them back as a map
	// we re parse them here to get their actual type back
	if tx.InMsg != nil {
		rawBody := hex.EncodeToString(tx.InMsg.Body)
		decodeMessageBody, err := decodeMessageBody(rawBody, tx.InMsg.MsgType)
		if err != nil {
			return nil, err
		}
		tx.InMsg.DecodedBody = decodeMessageBody
	}
	for i := range tx.OutMsgs {
		rawBody := hex.EncodeToString(tx.OutMsgs[i].Body)
		decodeMessageBody, err := decodeMessageBody(rawBody, tx.OutMsgs[i].MsgType)
		if err != nil {
			return nil, err
		}
		tx.OutMsgs[i].DecodedBody = decodeMessageBody
	}
	return &tx, nil
}

func (s *LiteStorage) searchParentTxInCache(message core.Message) *core.Transaction {
	hash, err := s.transactionsByOutMsg.Get(context.Background(), getKeyFromMessage(message))
	if err != nil {
		return nil
	}
	tx, err := s.getTxFromCacheByHash(hash)
	if err != nil {
		return nil
	}
	return tx
}

func (s *LiteStorage) searchChildTxInCache(message core.Message) *core.Transaction {
	hash, err := s.transactionsByInMsg.Get(context.Background(), getKeyFromMessage(message))
	if err != nil {
		return nil
	}
	tx, err := s.getTxFromCacheByHash(hash)
	if err != nil {
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

func (s *LiteStorage) GetAccountTransactions(ctx context.Context, id tongo.AccountID, limit int, beforeLt, afterLt uint64, descendingOrder bool) ([]*core.Transaction, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_account_transactions").Observe(v)
	}))
	defer timer.ObserveDuration()
	txs, err := s.client.GetLastTransactions(ctx, id, limit) //todo: custom with beforeLt, afterLt and descendingOrder
	if err != nil {
		return nil, err
	}
	result := make([]*core.Transaction, len(txs))
	for i := range txs {
		tx, err := core.ConvertTransaction(id.Workchain, txs[i], nil)
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
	_, result, err := abi.GetPublicKey(ctx, s.executor, address)
	if err == nil {
		if r, ok := result.(abi.GetPublicKeyResult); ok {
			i := big.Int(r.PublicKey)
			b := i.Bytes()
			if len(b) < 24 || len(b) > 32 {
				return nil, fmt.Errorf("invalid public key")
			}
			return append(make([]byte, 32-len(b)), b...), nil
		}
	}
	pubKey, ok := s.pubKeyByAccountID.Load(address)
	if ok {
		return pubKey, nil
	}
	return nil, fmt.Errorf("can't get public key")
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

func (c *LiteStorage) GetInscriptionBalancesByAccount(ctx context.Context, a ton.AccountID) ([]core.InscriptionBalance, error) {
	return nil, fmt.Errorf("not implemented") //and cannot be without full blockckchain index
}

func (c *LiteStorage) GetInscriptionsHistoryByAccount(ctx context.Context, a ton.AccountID, ticker *string, beforeLt int64, limit int) ([]core.InscriptionMessage, error) {
	return nil, fmt.Errorf("not implemented") //and cannot be without full blockckchain index
}

func (s *LiteStorage) GetReducedBlocks(ctx context.Context, from, to int64) ([]core.ReducedBlock, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *LiteStorage) GetAccountMultisigs(ctx context.Context, accountID ton.AccountID) ([]core.Multisig, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *LiteStorage) GetMultisigByID(ctx context.Context, accountID ton.AccountID) (*core.Multisig, error) {
	return nil, fmt.Errorf("not implemented")
}

func OasTransactionToCoreTransaction(oasTransaction tonapi.Transaction) (core.Transaction, error) {
	transactionId := core.TransactionID{
		Hash:    tongo.MustParseHash(oasTransaction.Hash),
		Lt:      uint64(oasTransaction.Lt),
		Account: tongo.MustParseAddress(oasTransaction.Account.GetAddress()).ID,
	}

	coreTransaction := core.Transaction{
		TransactionID: transactionId,
		Type:          core.TransactionType(oasTransaction.TransactionType),
		Success:       oasTransaction.Success,
		Utime:         oasTransaction.Utime,
		OrigStatus:    tlb.AccountStatus(oasTransaction.OrigStatus),
		EndStatus:     tlb.AccountStatus(oasTransaction.EndStatus),
		BlockID:       tongo.MustParseBlockID(oasTransaction.Block),
		EndBalance:    oasTransaction.EndBalance,
		Aborted:       oasTransaction.Aborted,
		Destroyed:     oasTransaction.Destroyed,
		TotalFee:      oasTransaction.TotalFees,
	}

	oldHash := tlb.Bits256{}
	bytes, err := hex.DecodeString(oasTransaction.StateUpdateOld)
	if err != nil {
		return core.Transaction{}, err
	}
	copy(oldHash[:], bytes)

	newHash := tlb.Bits256{}
	bytes, err = hex.DecodeString(oasTransaction.StateUpdateNew)
	if err != nil {
		return core.Transaction{}, err
	}
	copy(newHash[:], bytes)

	coreTransaction.StateHashUpdate = tlb.HashUpdate{
		OldHash: oldHash,
		NewHash: newHash,
	}

	coreTransaction.Raw, err = hex.DecodeString(oasTransaction.Raw)
	if err != nil {
		return core.Transaction{}, err
	}

	if oasTransaction.InMsg.Set {
		coreMessage, err := oasMessageToCoreMessage(oasTransaction.InMsg.Value)
		if err != nil {
			return core.Transaction{}, err
		}
		coreTransaction.InMsg = &coreMessage
	}
	for _, outMessage := range oasTransaction.OutMsgs {
		coreMessage, err := oasMessageToCoreMessage(outMessage)
		if err != nil {
			return core.Transaction{}, err
		}
		coreTransaction.OutMsgs = append(coreTransaction.OutMsgs, coreMessage)
	}

	if oasTransaction.PrevTransLt.Set && oasTransaction.PrevTransHash.Set {
		coreTransaction.PrevTransLt = uint64(oasTransaction.PrevTransLt.Value)
		coreTransaction.PrevTransHash = tongo.MustParseHash(oasTransaction.PrevTransHash.Value)
	}

	if oasTransaction.ComputePhase.Set {
		oasComputePhase := oasTransaction.ComputePhase.Value
		computePhase := core.TxComputePhase{
			Skipped: oasComputePhase.Skipped,
		}
		if oasComputePhase.Skipped {
			computePhase.SkipReason = tlb.ComputeSkipReason(oasComputePhase.SkipReason.Value)
		} else {
			computePhase.Success = oasComputePhase.Success.Value
			computePhase.GasFees = uint64(oasComputePhase.GasFees.Value)
			computePhase.GasUsed = *big.NewInt(oasComputePhase.GasUsed.Value)
			computePhase.VmSteps = uint32(oasComputePhase.VMSteps.Value)
			computePhase.ExitCode = int32(oasComputePhase.ExitCode.Value)
		}
		coreTransaction.ComputePhase = &computePhase
	}

	if oasTransaction.StoragePhase.Set {
		oasStoragePhase := oasTransaction.StoragePhase.Value
		storagePhase := core.TxStoragePhase{
			StorageFeesCollected: uint64(oasStoragePhase.FeesCollected),
			StatusChange:         tlb.AccStatusChange(oasStoragePhase.StatusChange),
		}
		if oasStoragePhase.FeesDue.Set {
			storageFeesDue := uint64(oasStoragePhase.FeesDue.Value)
			storagePhase.StorageFeesDue = &storageFeesDue
		}
		coreTransaction.StoragePhase = &storagePhase
	}

	if oasTransaction.CreditPhase.Set {
		oasCreditPhase := oasTransaction.CreditPhase.Value
		creditPhase := core.TxCreditPhase{
			DueFeesCollected: uint64(oasCreditPhase.FeesCollected),
			CreditGrams:      uint64(oasCreditPhase.Credit),
		}
		coreTransaction.CreditPhase = &creditPhase
	}

	if oasTransaction.ActionPhase.Set {
		oasActionPhase := oasTransaction.ActionPhase.Value
		actionPhase := core.TxActionPhase{
			Success:        oasActionPhase.Success,
			ResultCode:     int32(oasActionPhase.ResultCode),
			TotalActions:   uint16(oasActionPhase.TotalActions),
			SkippedActions: uint16(oasActionPhase.SkippedActions),
			FwdFees:        uint64(oasActionPhase.FwdFees),
			TotalFees:      uint64(oasActionPhase.TotalFees),
		}
		coreTransaction.ActionPhase = &actionPhase
	}

	if oasTransaction.BouncePhase.Set {
		oasBouncePhase := oasTransaction.BouncePhase.Value
		bouncePhase := core.TxBouncePhase{
			Type: core.BouncePhaseType(oasBouncePhase),
		}
		coreTransaction.BouncePhase = &bouncePhase
	}
	return coreTransaction, nil
}

func oasMessageToCoreMessage(oasMessage tonapi.Message) (core.Message, error) {
	messageId := core.MessageID{
		CreatedLt: uint64(oasMessage.CreatedLt),
	}
	if oasMessage.Source.Set {
		accountId := tongo.MustParseAddress(oasMessage.Source.Value.Address).ID
		messageId.Source = &accountId
	}
	if oasMessage.Destination.Set {
		accountId := tongo.MustParseAddress(oasMessage.Destination.Value.Address).ID
		messageId.Destination = &accountId
	}

	coreMessage := core.Message{
		MessageID:   messageId,
		Hash:        tongo.MustParseHash(oasMessage.Hash),
		IhrDisabled: oasMessage.IhrDisabled,
		Bounce:      oasMessage.Bounce,
		Bounced:     oasMessage.Bounced,
		Value:       oasMessage.Value,
		FwdFee:      oasMessage.FwdFee,
		IhrFee:      oasMessage.IhrFee,
		ImportFee:   oasMessage.ImportFee,
		CreatedAt:   uint32(oasMessage.CreatedAt),
	}

	if oasMessage.MsgType == tonapi.MessageMsgTypeExtInMsg {
		coreMessage.MsgType = core.ExtInMsg
	} else if oasMessage.MsgType == tonapi.MessageMsgTypeExtOutMsg {
		coreMessage.MsgType = core.ExtOutMsg
	} else {
		coreMessage.MsgType = core.IntMsg
	}

	if oasMessage.RawBody.Set {
		decodedMessageBody, err := decodeMessageBody(oasMessage.RawBody.Value, coreMessage.MsgType)
		if err == nil {
			coreMessage.DecodedBody = decodedMessageBody
		}
	}

	if oasMessage.Init.Set {
		coreMessage.Init, _ = hex.DecodeString(oasMessage.Init.Value.Boc)
		for _, iface := range oasMessage.Init.Value.Interfaces {
			coreMessage.InitInterfaces = append(coreMessage.InitInterfaces, abi.ContractInterfaceFromString(iface))
		}
	}

	if oasMessage.RawBody.Set {
		var err error
		coreMessage.Body, err = hex.DecodeString(oasMessage.RawBody.Value)
		if err != nil {
			return core.Message{}, err
		}
	}

	if oasMessage.OpCode.Set {
		bytes, err := hex.DecodeString(oasMessage.OpCode.Value[2:])
		if err != nil {
			return core.Message{}, err
		}
		opCode := binary.BigEndian.Uint32(bytes)
		coreMessage.OpCode = &opCode
	}
	return coreMessage, nil
}

func oasAccountToCoreAccount(oasAccount tonapi.BlockchainRawAccount) (core.Account, error) {
	coreAccount := core.Account{
		AccountAddress:    tongo.MustParseAddress(oasAccount.Address).ID,
		Status:            tlb.AccountStatus(oasAccount.Status),
		TonBalance:        oasAccount.Balance,
		LastTransactionLt: uint64(oasAccount.LastTransactionLt),
	}
	if oasAccount.Code.Set {
		var err error
		coreAccount.Code, err = hex.DecodeString(oasAccount.Code.Value)
		if err != nil {
			return core.Account{}, err
		}
	}
	return coreAccount, nil
}

func decodeMessageBody(rawBody string, msgType core.MsgType) (*core.DecodedMessageBody, error) {
	if rawBody == "" {
		return nil, nil
	}

	cells, err := boc.DeserializeBocHex(rawBody)
	if err != nil {
		return nil, err
	}
	if len(cells) != 1 {
		return nil, errors.New("multiple cells not supported")
	}
	cell := cells[0]

	var op *string
	var value any
	switch msgType {
	case "IntMsg":
		_, op, value, err = abi.InternalMessageDecoder(cell, nil)
	case "ExtInMsg":
		_, op, value, err = abi.ExtInMessageDecoder(cell, nil)
	case "ExtOutMsg":
		_, op, value, err = abi.ExtOutMessageDecoder(cell, nil, tlb.MsgAddress{})
	}
	if err != nil {
		return nil, err
	}
	if op != nil {
		return &core.DecodedMessageBody{Operation: *op, Value: value}, nil
	}
	return nil, nil
}
