package api

import (
	"fmt"

	"github.com/go-faster/errors"
	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/contract/dns"
	"github.com/tonkeeper/tongo/tep64"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/blockchain"
	"github.com/tonkeeper/opentonapi/pkg/cache"
	"github.com/tonkeeper/opentonapi/pkg/chainstate"
	"github.com/tonkeeper/opentonapi/pkg/config"
	"github.com/tonkeeper/opentonapi/pkg/image"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/rates"
)

// Compile-time check for Handler.
var _ oas.Handler = (*Handler)(nil)

type Handler struct {
	oas.UnimplementedHandler // automatically implement all methods

	addressBook      addressBook
	storage          storage
	state            chainState
	msgSender        messageSender
	previewGenerator previewGenerator
	executor         executor
	dns              *dns.DNS
	limits           Limits
	spamRules        func() rules.Rules
	tonRates         tonRates
	metaCache        metadataCache
}

// Options configures behavior of a Handler instance.
type Options struct {
	storage          storage
	chainState       chainState
	addressBook      addressBook
	msgSender        messageSender
	previewGenerator previewGenerator
	executor         executor
	limits           Limits
	spamRules        func() rules.Rules
	tonRates         tonRates
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

func WithPreviewGenerator(previewGenerator previewGenerator) Option {
	return func(o *Options) {
		o.previewGenerator = previewGenerator
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

func WithSpamRules(spamRules func() rules.Rules) Option {
	return func(o *Options) {
		o.spamRules = spamRules
	}
}

func NewHandler(logger *zap.Logger, opts ...Option) (*Handler, error) {
	options := &Options{}
	for _, o := range opts {
		o(options)
	}
	if options.msgSender == nil {
		sender, err := blockchain.NewMsgSender(logger)
		if err != nil {
			return nil, err
		}
		options.msgSender = sender
	}
	if options.addressBook == nil {
		options.addressBook = addressbook.NewAddressBook(logger, config.AddressPath, config.JettonPath, config.CollectionPath)
	}
	if options.chainState == nil {
		options.chainState = chainstate.NewChainState()
	}
	if options.storage == nil {
		return nil, errors.New("storage is not configured")
	}
	if options.previewGenerator == nil {
		options.previewGenerator = image.NewImgGenerator()
	}
	if options.spamRules == nil {
		options.spamRules = func() rules.Rules {
			defaultRules := rules.GetDefaultRules()
			return defaultRules
		}
	}
	if options.tonRates == nil {
		options.tonRates = rates.InitTonRates(logger)
	}
	if options.executor == nil {
		return nil, fmt.Errorf("executor is not configured")
	}
	dnsClient := dns.NewDNS(tongo.MustParseAccountID("-1:e56754f83426f69b09267bd876ac97c44821345b7e266bd956a7bfbfb98df35c"), options.executor) //todo: move to chain config

	return &Handler{
		storage:          options.storage,
		state:            options.chainState,
		addressBook:      options.addressBook,
		msgSender:        options.msgSender,
		previewGenerator: options.previewGenerator,
		executor:         options.executor,
		dns:              dnsClient,
		limits:           options.limits,
		spamRules:        options.spamRules,
		tonRates:         options.tonRates,
		metaCache: metadataCache{
			collectionsCache: cache.NewLRUCache[tongo.AccountID, tep64.Metadata](10000, "nft_metadata_cache"),
			jettonsCache:     cache.NewLRUCache[tongo.AccountID, tep64.Metadata](10000, "jetton_metadata_cache"),
			storage:          options.storage,
		},
	}, nil
}
