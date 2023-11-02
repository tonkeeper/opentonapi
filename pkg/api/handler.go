package api

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-faster/errors"
	"github.com/tonkeeper/opentonapi/pkg/chainstate"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/rates"
	"github.com/tonkeeper/opentonapi/pkg/spam"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/contract/dns"
	"github.com/tonkeeper/tongo/tep64"
	"github.com/tonkeeper/tongo/tonconnect"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/cache"
	"github.com/tonkeeper/opentonapi/pkg/config"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

// Compile-time check for Handler.
var _ oas.Handler = (*Handler)(nil)

type Handler struct {
	logger *zap.Logger

	addressBook    addressBook
	storage        storage
	state          chainState
	msgSender      messageSender
	executor       executor
	limits         Limits
	spamFilter     spamFilter
	ratesSource    ratesSource
	metaCache      metadataCache
	mempoolEmulate mempoolEmulate
	tonConnect     *tonconnect.Server

	// mu protects "dns".
	mu  sync.Mutex
	dns *dns.DNS // todo: update when blockchain config changes
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
	spamFilter       spamFilter
	ratesSource      ratesSource
	tonConnectSecret string
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

func WithSpamFilter(spamFilter spamFilter) Option {
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

func NewHandler(logger *zap.Logger, opts ...Option) (*Handler, error) {
	options := &Options{}
	for _, o := range opts {
		o(options)
	}
	if options.msgSender == nil {
		logger.Warn("message sender is not configured, you can't send messages to the blockchain")
	}
	if options.addressBook == nil {
		options.addressBook = addressbook.NewAddressBook(logger, config.AddressPath, config.JettonPath, config.CollectionPath)
	}
	if options.storage == nil {
		return nil, errors.New("storage is not configured")
	}
	if options.chainState == nil {
		options.chainState = chainstate.NewChainState(options.storage)
	}
	if options.spamFilter == nil {
		options.spamFilter = spam.NewNoOpSpamFilter()
	}
	if options.ratesSource == nil {
		options.ratesSource = rates.Mock{Storage: options.storage}
	}
	if options.executor == nil {
		return nil, fmt.Errorf("executor is not configured")
	}
	tonConnect, err := tonconnect.NewTonConnect(options.executor, options.tonConnectSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to init tonconnect")
	}
	return &Handler{
		logger:      logger,
		storage:     options.storage,
		state:       options.chainState,
		addressBook: options.addressBook,
		msgSender:   options.msgSender,
		executor:    options.executor,
		limits:      options.limits,
		spamFilter:  options.spamFilter,
		ratesSource: rates.InitCalculator(options.ratesSource),
		metaCache: metadataCache{
			collectionsCache: cache.NewLRUCache[tongo.AccountID, tep64.Metadata](10000, "nft_metadata_cache"),
			jettonsCache:     cache.NewLRUCache[tongo.AccountID, tep64.Metadata](10000, "jetton_metadata_cache"),
			storage:          options.storage,
		},
		mempoolEmulate: mempoolEmulate{
			traces:         cache.NewLRUCache[string, *core.Trace](10000, "mempool_traces_cache"),
			accountsTraces: cache.NewLRUCache[tongo.AccountID, []string](10000, "accounts_traces_cache"),
		},
		tonConnect: tonConnect,
	}, nil
}

func (h *Handler) GetJettonNormalizedMetadata(ctx context.Context, master tongo.AccountID) NormalizedMetadata {
	meta, _ := h.metaCache.getJettonMeta(ctx, master)
	// TODO: should we ignore the second returned value?
	if info, ok := h.addressBook.GetJettonInfoByAddress(master); ok {
		return NormalizeMetadata(meta, &info, false)
	}
	return NormalizeMetadata(meta, nil, h.spamFilter.IsJettonBlacklisted(master, meta.Symbol))
}
