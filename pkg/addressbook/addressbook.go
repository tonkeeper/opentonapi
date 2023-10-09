package addressbook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/shurcooL/graphql"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

// KnownAddress represents additional manually crafted information about a particular account in the blockchain.
type KnownAddress struct {
	IsScam      bool   `json:"is_scam,omitempty"`
	RequireMemo bool   `json:"require_memo,omitempty"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	Image       string `json:"image,omitempty"`
}

// AttachedAccount represents domains, nft collections for quick search by name are presented
type AttachedAccount struct {
	Name       string `json:"name"`
	Preview    string `json:"preview"`
	Wallet     string `json:"wallet"`
	Symbol     string `json:"-"`
	Normalized string `json:"-"`
}

type JettonVerificationType string

const (
	Whitelist JettonVerificationType = "whitelist"
	Blacklist JettonVerificationType = "blacklist"
	None      JettonVerificationType = "none"
)

// KnownJetton represents additional manually crafted information about a particular jetton in the blockchain.
type KnownJetton struct {
	Name          string                 `json:"name"`
	Verification  JettonVerificationType `json:"verification"`
	Description   string                 `json:"description"`
	Image         string                 `json:"image"`
	Address       string                 `json:"address"`
	Symbol        string                 `json:"symbol"`
	MaxSupply     decimal.Decimal        `json:"max_supply"`
	Websites      []string               `json:"websites,omitempty"`
	Social        []string               `json:"social,omitempty"`
	Coinmarketcap string                 `json:"coinmarketcap,omitempty"`
	Coingecko     string                 `json:"coingecko,omitempty"`
}

// KnownCollection represents additional manually crafted information about a particular NFT collection in the blockchain.
type KnownCollection struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Address     string   `json:"address"`
	MaxItems    int64    `json:"max_items"`
	Websites    []string `json:"websites,omitempty"`
	Social      []string `json:"social,omitempty"`
	Approvers   []oas.NftItemApprovedByItem
}

type Option func(o *Options)

type Options struct {
	addressers []addresser
}

type addresser interface {
	GetAddress(a tongo.AccountID) (KnownAddress, bool)
	SearchAttachedAccounts(prefix string) []AttachedAccount
}

func WithAdditionalAddressesSource(a addresser) Option {
	return func(o *Options) {
		o.addressers = append(o.addressers, a)
	}
}

// Book holds information about known accounts, jettons, NFT collections manually crafted by the tonkeeper team and the community.
type Book struct {
	mu          sync.RWMutex
	addresses   map[tongo.AccountID]KnownAddress
	collections map[tongo.AccountID]KnownCollection
	jettons     map[tongo.AccountID]KnownJetton
	tfPools     map[tongo.AccountID]TFPoolInfo
	addressers  []addresser
}

type TFPoolInfo struct {
	Name      string `json:"name"`
	GroupName string `json:"groupName"`
	Address   string `json:"address"`
}

func (b *Book) GetAddressInfoByAddress(a tongo.AccountID) (KnownAddress, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if a1, ok := b.addresses[a]; ok {
		return a1, ok
	}
	for i := range b.addressers {
		if a1, ok := b.addressers[i].GetAddress(a); ok {
			return a1, ok
		}
	}
	return KnownAddress{}, false
}

func (b *Book) SearchAttachedAccountsByPrefix(prefix string) []AttachedAccount {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for i := range b.addressers {
		foundAccounts := b.addressers[i].SearchAttachedAccounts(prefix)
		if len(foundAccounts) > 0 {
			return foundAccounts
		}
	}
	return []AttachedAccount{}
}

func (b *Book) GetTFPoolInfo(a tongo.AccountID) (TFPoolInfo, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	info, ok := b.tfPools[a]
	return info, ok
}

func (b *Book) GetKnownCollections() map[tongo.AccountID]KnownCollection {
	b.mu.RLock()
	defer b.mu.RUnlock()

	collections := make(map[tongo.AccountID]KnownCollection, len(b.collections))
	for accountID, collection := range b.collections {
		collections[accountID] = collection
	}
	return collections
}

func (b *Book) GetCollectionInfoByAddress(a tongo.AccountID) (KnownCollection, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	c, ok := b.collections[a]
	return c, ok
}

func (b *Book) GetKnownJettons() map[tongo.AccountID]KnownJetton {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return maps.Clone(b.jettons)
}

func (b *Book) GetJettonInfoByAddress(a tongo.AccountID) (KnownJetton, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	j, ok := b.jettons[a]
	if ok {
		j.Verification = Whitelist
	} else {
		j.Verification = None
	}
	return j, ok
}

func (b *Book) TFPools() []tongo.AccountID {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return maps.Keys(b.tfPools)
}

func NewAddressBook(logger *zap.Logger, addressPath, jettonPath, collectionPath string, opts ...Option) *Book {
	var options Options
	for _, opt := range opts {
		opt(&options)
	}
	addresses := make(map[tongo.AccountID]KnownAddress)
	collections := make(map[tongo.AccountID]KnownCollection)
	jettons := make(map[tongo.AccountID]KnownJetton)
	tfPools := make(map[tongo.AccountID]TFPoolInfo)

	book := &Book{
		addresses:   addresses,
		collections: collections,
		jettons:     jettons,
		tfPools:     tfPools,
		addressers:  options.addressers,
	}

	go func() {
		for {
			book.refresh(logger, addressPath, jettonPath, collectionPath)
			time.Sleep(time.Minute * 10)
		}
	}()

	go book.getGGWhitelist(logger)
	go book.getTonDiamondsWhitelist()

	return book
}

func (b *Book) refresh(logger *zap.Logger, addressPath, jettonPath, collectionPath string) {
	go b.refreshAddresses(logger, addressPath)
	go b.refreshJettons(logger, jettonPath)
	go b.refreshCollections(logger, collectionPath)
	go b.refreshTfPools(logger)
}

func (b *Book) refreshAddresses(logger *zap.Logger, addressPath string) {
	addresses, err := downloadJson[KnownAddress](addressPath)
	if err != nil {
		logger.Info("failed to load accounts.json")
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, item := range addresses {
		accountID, err := tongo.ParseAccountID(item.Address)
		if err != nil {
			continue
		}
		item.Address = accountID.ToRaw()
		b.addresses[accountID] = item
	}
}

func (b *Book) refreshJettons(logger *zap.Logger, jettonPath string) {
	jettons, err := downloadJson[KnownJetton](jettonPath)
	if err != nil {
		logger.Info("failed to load jettons.json")
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, item := range jettons {
		accountID, err := tongo.ParseAccountID(item.Address)
		if err != nil {
			continue
		}
		item.Address = accountID.ToRaw()
		b.jettons[accountID] = item
	}
}

func unique(approvers []oas.NftItemApprovedByItem) []oas.NftItemApprovedByItem {
	sort.Slice(approvers, func(i, j int) bool {
		return approvers[i] < approvers[j]
	})
	return slices.Compact(approvers)
}

func (b *Book) refreshCollections(logger *zap.Logger, collectionPath string) {
	collections, err := downloadJson[KnownCollection](collectionPath)
	if err != nil {
		logger.Info("fail to load collections.json")
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, item := range collections {
		// TODO: remove items that were previously added but aren't present in the current list.
		accountID, err := tongo.ParseAccountID(item.Address)
		if err != nil {
			continue
		}
		currentCollection, ok := b.collections[accountID]
		if !ok {
			// this is a new item, so we only add tonkeeper as approver.
			item.Address = accountID.ToRaw()
			item.Approvers = unique(append(item.Approvers, oas.NftItemApprovedByItemTonkeeper))
			b.collections[accountID] = item
			continue
		}
		// this is an existing item, so we merge approvers and remove duplicates adding tonkeeper.
		item.Approvers = unique(append(append(currentCollection.Approvers, item.Approvers...), oas.NftItemApprovedByItemTonkeeper))
		b.collections[accountID] = item
	}
}

func (b *Book) refreshTfPools(logger *zap.Logger) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, pool := range getPools(logger) {
		accountID, err := tongo.ParseAccountID(pool.Address)
		if err != nil {
			logger.Error("failed to parse account in pools", zap.Error(err))
			continue
		}
		pool.Address = accountID.ToRaw()
		b.tfPools[accountID] = pool
	}
}

func downloadJson[T any](url string) ([]T, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return nil, fmt.Errorf("invalid status code %v", response.StatusCode)
	}
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	var data []T
	if err = json.Unmarshal(content, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (b *Book) getGGWhitelist(logger *zap.Logger) {
	client := graphql.NewClient("https://api.getgems.io/graphql", nil)
	for {
		if len(b.GetKnownCollections()) == 0 {
			time.Sleep(time.Second * 10)
			continue
		}
		addresses, err := _getGGWhitelist(client)
		if err != nil {
			logger.Warn(fmt.Sprintf("get nft collection whitelist: %v", err))
			time.Sleep(time.Minute * 3)
			continue
		}
		b.mu.Lock()
		for _, account := range addresses {
			collection := b.collections[account]
			collection.Approvers = unique(append(collection.Approvers, oas.NftItemApprovedByItemGetgems))
			b.collections[account] = collection
		}
		b.mu.Unlock()
		return
	}
}

func (b *Book) getTonDiamondsWhitelist() {
	for {
		if len(b.GetKnownCollections()) == 0 {
			time.Sleep(time.Second * 10)
			continue
		}
		b.mu.Lock()
		for _, account := range references.TonDiamondsCollections {
			collection := b.collections[account]
			collection.Approvers = unique(append(collection.Approvers, oas.NftItemApprovedByItemTonDiamonds))
			b.collections[account] = collection
		}
		b.mu.Unlock()
		return
	}
}

func _getGGWhitelist(client *graphql.Client) ([]tongo.AccountID, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var q struct {
		GetAddressesOfVerifiedCollections []graphql.String `graphql:"getAddressesOfVerifiedCollections"`
	}
	err := client.Query(ctx, &q, nil)
	if err != nil {
		return nil, err
	}
	var addr []tongo.AccountID
	for _, collection := range q.GetAddressesOfVerifiedCollections {
		aa, err := tongo.ParseAccountID(string(collection))
		if err != nil {
			return nil, err
		}
		addr = append(addr, aa)
	}
	return addr, nil
}
