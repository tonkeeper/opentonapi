package api

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/exp/slog"

	"github.com/go-faster/errors"
	"github.com/tonkeeper/opentonapi/pkg/chainstate"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/rates"
	"github.com/tonkeeper/opentonapi/pkg/score"
	"github.com/tonkeeper/opentonapi/pkg/verifier"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/contract/dns"
	"github.com/tonkeeper/tongo/tep64"
	"github.com/tonkeeper/tongo/ton"
	"github.com/tonkeeper/tongo/tonconnect"
	"github.com/tonkeeper/tongo/tvm"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/cache"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

// Compile-time check for Handler.
var _ oas.Handler = (*Handler)(nil)

// ctxToDetails converts a request context to a details instance.
type ctxToDetails func(ctx context.Context) any

type Handler struct {
	logger *zap.Logger

	addressBook addressBook
	storage     storage
	state       chainState
	msgSender   messageSender
	executor    executor
	gasless     Gasless

	limits         Limits
	spamFilter     SpamFilter
	ratesSource    ratesSource
	score          scoreSource
	metaCache      metadataCache
	tonConnect     *tonconnect.Server
	verifierSource verifierSource

	// mempoolEmulate contains results of emulation of messages that are in the mempool.
	mempoolEmulate mempoolEmulate
	// ctxToDetails converts a request context to a details instance.
	ctxToDetails ctxToDetails
	tongoVersion int

	// mempoolEmulateIgnoreAccounts, we don't track pending transactions for this list of accounts.
	mempoolEmulateIgnoreAccounts map[tongo.AccountID]struct{}

	// need to blacklist BoCs for avoiding spamming
	blacklistedBocCache cache.Cache[[32]byte, struct{}]

	// getMethodsCache contains results of methods.
	getMethodsCache cache.Cache[string, *oas.MethodExecutionResult]

	// mu protects "dns".
	mu         sync.Mutex
	dns        *dns.DNS // todo: update when blockchain config changes
	configPool *sync.Pool
}

func (h *Handler) NewError(ctx context.Context, err error) *oas.ErrorStatusCode {
	return new(oas.ErrorStatusCode)
}

// Options configures behavior of a Handler instance.
type Options struct {
	storage          storage
	chainState       chainState
	addressBook      addressBook
	msgSender        messageSender
	executor         executor
	limits           Limits
	spamFilter       SpamFilter
	ratesSource      ratesSource
	tonConnectSecret string
	ctxToDetails     ctxToDetails
	gasless          Gasless
	verifier         verifierSource
	score            scoreSource
}

type Option func(o *Options)

func WithStorage(s storage) Option {
	return func(o *Options) {
		o.storage = s
	}
}
func WithChainState(state chainState) Option {
	return func(o *Options) {
		o.chainState = state
	}
}

func WithAddressBook(book addressBook) Option {
	return func(o *Options) {
		o.addressBook = book
	}
}

func WithMessageSender(msgSender messageSender) Option {
	return func(o *Options) {
		o.msgSender = msgSender
	}
}

func WithExecutor(e executor) Option {
	return func(o *Options) {
		o.executor = e
	}
}

func WithLimits(limits Limits) Option {
	return func(o *Options) {
		o.limits = limits
	}
}

func WithSpamFilter(spamFilter SpamFilter) Option {
	return func(o *Options) {
		o.spamFilter = spamFilter
	}
}

func WithRatesSource(source ratesSource) Option {
	return func(o *Options) {
		o.ratesSource = source
	}
}

func WithTonConnectSecret(tonConnectSecret string) Option {
	return func(o *Options) {
		o.tonConnectSecret = tonConnectSecret
	}
}

func WithContextToDetails(ctxToDetails ctxToDetails) Option {
	return func(o *Options) {
		o.ctxToDetails = ctxToDetails
	}
}

func WithGasless(gasless Gasless) Option {
	return func(o *Options) {
		o.gasless = gasless
	}
}

func WithVerifier(verifier verifierSource) Option {
	return func(o *Options) {
		o.verifier = verifier
	}
}

func WithScore(score scoreSource) Option {
	return func(o *Options) {
		o.score = score
	}
}

func NewHandler(logger *zap.Logger, opts ...Option) (*Handler, error) {
	options := &Options{}
	for _, o := range opts {
		o(options)
	}
	if options.msgSender == nil {
		logger.Warn("message sender is not configured, you can't send messages to the blockchain")
	}
	if options.storage == nil {
		return nil, errors.New("storage is not configured")
	}
	if options.addressBook == nil {
		return nil, errors.New("address book is not configured")
	}
	if options.chainState == nil {
		options.chainState = chainstate.NewChainState(options.storage)
	}
	if options.ratesSource == nil {
		options.ratesSource = rates.Mock{}
	}
	if options.executor == nil {
		return nil, fmt.Errorf("executor is not configured")
	}
	if options.ctxToDetails == nil {
		options.ctxToDetails = func(ctx context.Context) any {
			return nil
		}
	}
	configBase64, err := options.storage.TrimmedConfigBase64()
	if err != nil {
		return nil, err
	}
	if options.verifier == nil {
		options.verifier = verifier.NewVerifier()
	}
	configPool := &sync.Pool{
		New: func() interface{} {
			config, err := tvm.CreateConfig(configBase64)
			if err != nil {
				return nil
			}
			return config
		},
	}
	tonConnect, err := tonconnect.NewTonConnect(options.executor, options.tonConnectSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to init tonconnect")
	}
	if options.score == nil {
		options.score = score.NewScore()
	}
	tongoVersion, err := GetPackageVersionInt("tongo")
	if err != nil {
		slog.Warn("unable to detect tongo version", "err", err)
	}
	return &Handler{
		logger:         logger,
		storage:        options.storage,
		state:          options.chainState,
		addressBook:    options.addressBook,
		msgSender:      options.msgSender,
		executor:       options.executor,
		limits:         options.limits,
		spamFilter:     options.spamFilter,
		ctxToDetails:   options.ctxToDetails,
		gasless:        options.gasless,
		score:          options.score,
		ratesSource:    rates.InitCalculator(options.ratesSource),
		verifierSource: options.verifier,
		metaCache: metadataCache{
			collectionsCache: cache.NewLRUCache[tongo.AccountID, tep64.Metadata](10000, "nft_metadata_cache"),
			jettonsCache:     cache.NewLRUCache[tongo.AccountID, tep64.Metadata](10000, "jetton_metadata_cache"),
			storage:          options.storage,
		},
		mempoolEmulate: mempoolEmulate{
			accountsTraces: cache.NewLRUCache[tongo.AccountID, []ton.Bits256](10000, "accounts_traces_cache"),
		},
		mempoolEmulateIgnoreAccounts: map[tongo.AccountID]struct{}{
			tongo.MustParseAddress("0:0000000000000000000000000000000000000000000000000000000000000000").ID: {},
		},
		tongoVersion:        tongoVersion,
		blacklistedBocCache: cache.NewLRUCache[[32]byte, struct{}](100000, "blacklisted_boc_cache"),
		getMethodsCache:     cache.NewLRUCache[string, *oas.MethodExecutionResult](100000, "get_methods_cache"),
		tonConnect:          tonConnect,
		configPool:          configPool,
	}, nil
}

func (h *Handler) GetJettonNormalizedMetadata(ctx context.Context, master tongo.AccountID) NormalizedMetadata {
	meta, _ := h.metaCache.getJettonMeta(ctx, master)
	// TODO: should we ignore the second returned value?
	if info, ok := h.addressBook.GetJettonInfoByAddress(master); ok {
		return NormalizeMetadata(master, meta, &info, core.TrustNone)
	}
	return NormalizeMetadata(master, meta, nil, h.spamFilter.JettonTrust(master, meta.Symbol, meta.Name, meta.Image))
}
