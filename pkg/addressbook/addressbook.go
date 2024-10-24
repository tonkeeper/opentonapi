package addressbook

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tonkeeper/opentonapi/pkg/core"
	imgGenerator "github.com/tonkeeper/opentonapi/pkg/image"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/shurcooL/graphql"
	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"github.com/tonkeeper/opentonapi/pkg/cache"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

// KnownAddress represents additional manually crafted information about a particular account in the blockchain.
type KnownAddress struct {
	IsScam      bool   `json:"is_scam,omitempty"`
	RequireMemo bool   `json:"require_memo,omitempty"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	Image       string `json:"image,omitempty"`
}

type AttachedAccountType string

const (
	ManualAccountType        AttachedAccountType = "manual"
	NftCollectionAccountType AttachedAccountType = "collection"
	NftItemAccountType       AttachedAccountType = "nft"
	TonDomainAccountType     AttachedAccountType = "ton_domain"
	TgDomainAccountType      AttachedAccountType = "tg_domain"
	JettonSymbolAccountType  AttachedAccountType = "jetton_symbol"
	JettonNameAccountType    AttachedAccountType = "jetton_name"
)

// AttachedAccount represents domains, nft collections for quick search by name are presented
type AttachedAccount struct {
	Name       string              `json:"name"`
	Preview    string              `json:"preview"`
	Wallet     ton.AccountID       `json:"wallet"`
	Symbol     string              `json:"-"`
	Type       AttachedAccountType `json:"-"`
	Weight     int32               `json:"-"`
	Popular    int32               `json:"-"`
	Normalized string              `json:"-"`
}

// KnownJetton represents additional manually crafted information about a particular jetton in the blockchain.
type KnownJetton struct {
	Name          string          `json:"name"`
	Verification  core.TrustType  `json:"verification"`
	Description   string          `json:"description"`
	Image         string          `json:"image"`
	Address       string          `json:"address"`
	Symbol        string          `json:"symbol"`
	MaxSupply     decimal.Decimal `json:"max_supply"`
	Websites      []string        `json:"websites,omitempty"`
	Social        []string        `json:"social,omitempty"`
	Coinmarketcap string          `json:"coinmarketcap,omitempty"`
	Coingecko     string          `json:"coingecko,omitempty"`
}

// KnownCollection represents additional manually crafted information about a particular NFT collection in the blockchain.
type KnownCollection struct {
	Address   string `json:"address"`
	Approvers []oas.NftApprovedByItem
}

type Option func(o *Options)

type Options struct {
	addressers []addresser
}

type addresser interface {
	GetAddress(a tongo.AccountID) (KnownAddress, bool)
	SearchAttachedAccounts(prefix string) []AttachedAccount
}

type accountsStatesSource interface {
	AccountStatusAndInterfaces(a tongo.AccountID) (tlb.AccountStatus, []abi.ContractInterface, error)
}

func WithAdditionalAddressesSource(a addresser) Option {
	return func(o *Options) {
		o.addressers = append(o.addressers, a)
	}
}

// Book holds information about known accounts, jettons, NFT collections manually crafted by the tonkeeper team and the community.
type Book struct {
	addressers []addresser

	states          accountsStatesSource
	mu              sync.RWMutex
	collections     map[tongo.AccountID]KnownCollection
	jettons         map[tongo.AccountID]KnownJetton
	tfPools         map[tongo.AccountID]TFPoolInfo
	walletsResolved cache.Cache[tongo.AccountID, bool]
}

type TFPoolInfo struct {
	Name      string `json:"name"`
	GroupName string `json:"groupName"`
	Address   string `json:"address"`
}

func (b *Book) GetAddressInfoByAddress(a tongo.AccountID) (KnownAddress, bool) {
	for i := range b.addressers {
		if a1, ok := b.addressers[i].GetAddress(a); ok {
			return a1, ok
		}
	}
	return KnownAddress{}, false
}

func (b *Book) SearchAttachedAccountsByPrefix(prefix string) []AttachedAccount {
	prefix = strings.ToLower(normalizeReg.ReplaceAllString(prefix, ""))
	var accounts []AttachedAccount
	for i := range b.addressers {
		foundAccounts := b.addressers[i].SearchAttachedAccounts(prefix)
		if len(foundAccounts) > 0 {
			accounts = append(accounts, foundAccounts...)
		}
	}
	tonDomainPrefix := prefix + "ton"
	tgDomainPrefix := prefix + "tme"

	for i := range accounts {
		if accounts[i].Normalized == prefix || accounts[i].Normalized == tonDomainPrefix || accounts[i].Normalized == tgDomainPrefix {
			accounts[i].Weight *= 100 // full match
		}
	}
	sort.Slice(accounts, func(i, j int) bool {
		if accounts[i].Weight == accounts[j].Weight {
			return len(accounts[i].Name) < len(accounts[j].Name)
		}
		return accounts[i].Weight > accounts[j].Weight
	})
	if len(accounts) > 50 {
		accounts = accounts[:50]
	}
	return accounts
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
		j.Verification = core.TrustWhitelist
	} else {
		j.Verification = core.TrustNone
	}
	return j, ok
}

func (b *Book) TFPools() []tongo.AccountID {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return maps.Keys(b.tfPools)
}

// IsWallet returns true if the given address is a wallet.
func (b *Book) IsWallet(addr tongo.AccountID) (bool, error) {
	if wallet, ok := b.walletsResolved.Get(addr); ok {
		return wallet, nil
	}
	status, interfaces, err := b.states.AccountStatusAndInterfaces(addr)
	if err != nil {
		return false, fmt.Errorf("failed to figure out if %v is a wallet %w", addr, err)
	}
	if status == tlb.AccountNone || status == tlb.AccountUninit {
		b.walletsResolved.Set(addr, true, cache.WithExpiration(time.Minute)) //shorter period for non-existing accounts
		return true, nil
	}
	isWallet := false
	for _, i := range interfaces {
		if i.Implements(abi.Wallet) {
			isWallet = true
			break
		}
	}
	b.walletsResolved.Set(addr, isWallet, cache.WithExpiration(time.Hour))
	return isWallet, nil
}

type manyalAddresser struct {
	mu        sync.RWMutex
	addresses map[tongo.AccountID]KnownAddress
	sorted    []AttachedAccount
}

func (m *manyalAddresser) GetAddress(a tongo.AccountID) (KnownAddress, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	addr, ok := m.addresses[a]
	return addr, ok
}

func (m *manyalAddresser) SearchAttachedAccounts(prefix string) []AttachedAccount {
	m.mu.RLock()
	sortedList := m.sorted
	m.mu.RUnlock()
	startIdx, endIdx := findIndexes(sortedList, prefix)
	if startIdx == -1 || endIdx == -1 {
		return nil
	}
	foundAccounts := make([]AttachedAccount, endIdx-startIdx+1)
	copy(foundAccounts, sortedList[startIdx:endIdx+1])
	return foundAccounts
}

var normalizeReg = regexp.MustCompile("[^\\p{L}\\p{N}]")

func (m *manyalAddresser) refreshAddresses(addressPath string) error {
	addresses, err := downloadJson[KnownAddress](addressPath)
	if err != nil {
		return err
	}
	newAddresses := make(map[tongo.AccountID]KnownAddress, len(addresses))
	newSorted := []AttachedAccount{
		{Name: "The Locker", Wallet: ton.MustParseAccountID("EQDtFpEwcFAEcRe5mLVh2N6C0x-_hJEM7W61_JLnSF74p4q2"), Normalized: "locker", Weight: 1000},
		{Name: "The Locker", Wallet: ton.MustParseAccountID("EQDtFpEwcFAEcRe5mLVh2N6C0x-_hJEM7W61_JLnSF74p4q2"), Normalized: "thelocker", Weight: 10000},
		{Name: "Ton Foundation", Wallet: ton.MustParseAccountID("EQCLyZHP4Xe8fpchQz76O-_RmUhaVc_9BAoGyJrwJrcbz2eZ"), Normalized: "foundation", Weight: 1000},
	}
	for _, item := range addresses {
		account, err := tongo.ParseAddress(item.Address)
		if err != nil {
			continue
		}
		item.Address = account.ID.ToRaw()
		newAddresses[account.ID] = item
		var preview string
		if item.Image != "" {
			preview = imgGenerator.DefaultGenerator.GenerateImageUrl(item.Image, 200, 200)
		}
		newSorted = append(newSorted, AttachedAccount{
			Name:       item.Name,
			Preview:    preview,
			Wallet:     account.ID,
			Type:       ManualAccountType,
			Weight:     1000,
			Popular:    1,
			Normalized: strings.ToLower(normalizeReg.ReplaceAllString(item.Name, "")),
		})
	}
	sort.Slice(newSorted, func(i, j int) bool {
		return newSorted[i].Normalized < newSorted[j].Normalized
	})
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addresses = newAddresses
	m.sorted = newSorted
	return nil
}

func findIndexes(sortedList []AttachedAccount, prefix string) (int, int) {
	low := 0
	high := len(sortedList) - 1
	startIdx := -1
	for low <= high { // Find the starting index
		med := (low + high) / 2
		if strings.HasPrefix(sortedList[med].Normalized, prefix) {
			startIdx = med
			high = med - 1
		} else if sortedList[med].Normalized < prefix {
			low = med + 1
		} else {
			high = med - 1
		}
	}

	if startIdx == -1 { // Prefix not found
		return -1, -1
	}

	low = startIdx
	high = len(sortedList) - 1
	endIdx := -1
	for low <= high { // Find the ending index
		med := (low + high) / 2
		if strings.HasPrefix(sortedList[med].Normalized, prefix) {
			endIdx = med
			low = med + 1
		} else {
			high = med - 1
		}
	}

	return startIdx, endIdx
}

func NewAddressBook(logger *zap.Logger, addressPath, jettonPath, collectionPath string, storage accountsStatesSource, opts ...Option) *Book {
	var manual = &manyalAddresser{
		addresses: make(map[tongo.AccountID]KnownAddress),
	}
	options := Options{addressers: []addresser{manual}}
	for _, opt := range opts {
		opt(&options)
	}
	collections := make(map[tongo.AccountID]KnownCollection)
	jettons := make(map[tongo.AccountID]KnownJetton)
	tfPools := make(map[tongo.AccountID]TFPoolInfo)

	book := &Book{
		states:          storage,
		collections:     collections,
		jettons:         jettons,
		tfPools:         tfPools,
		addressers:      options.addressers,
		walletsResolved: cache.NewLRUCache[tongo.AccountID, bool](1_000_000, "is_wallet"),
	}

	go refresher("gg whitelist", time.Hour, 5*time.Minute, logger, book.getGGWhitelist)
	go refresher("addresses", time.Minute*15, 5*time.Minute, logger, func() error { return manual.refreshAddresses(addressPath) })
	go refresher("jettons", time.Minute*15, 5*time.Minute, logger, func() error { return book.refreshJettons(jettonPath) })
	go refresher("collections", time.Minute*15, 5*time.Minute, logger, func() error { return book.refreshCollections(collectionPath) })
	book.refreshTfPools(logger) //hardcoded so don't need to be refreshed

	return book
}

func refresher(name string, interval, errorInterval time.Duration, logger *zap.Logger, f func() error) {
	for {
		err := f()
		if err != nil {
			logger.Error("refresh "+name, zap.Error(err))
			time.Sleep(errorInterval + time.Duration(rand.Intn(10))*time.Second)
			continue
		}
		time.Sleep(interval + time.Duration(rand.Intn(10))*time.Second)
	}
}

func (b *Book) refreshJettons(jettonPath string) error {
	jettons, err := downloadJson[KnownJetton](jettonPath)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, item := range jettons {
		account, err := tongo.ParseAddress(item.Address)
		if err != nil {
			continue
		}
		item.Address = account.ID.ToRaw()
		b.jettons[account.ID] = item
	}
	return nil
}

func (b *Book) refreshCollections(collectionPath string) error {
	collections, err := downloadJson[KnownCollection](collectionPath)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, item := range collections {
		// TODO: remove items that were previously added but aren't present in the current list.
		account, err := tongo.ParseAddress(item.Address)
		if err != nil {
			continue
		}
		currentCollection, ok := b.collections[account.ID]
		if !ok {
			// this is a new item, so we only add tonkeeper as approver.
			currentCollection.Address = account.ID.ToRaw()
			currentCollection.Approvers = []oas.NftApprovedByItem{oas.NftApprovedByItemTonkeeper}
			b.collections[account.ID] = currentCollection
			continue
		}
		// this is an existing item, so we merge approvers and remove duplicates adding tonkeeper.
		if !slices.Contains(currentCollection.Approvers, oas.NftApprovedByItemTonkeeper) {
			currentCollection.Approvers = append(item.Approvers, oas.NftApprovedByItemTonkeeper)
			b.collections[account.ID] = currentCollection
		}
	}
	return nil
}

func (b *Book) refreshTfPools(logger *zap.Logger) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, pool := range getPools(logger) {
		account, err := tongo.ParseAddress(pool.Address)
		if err != nil {
			logger.Error("failed to parse account in pools", zap.Error(err))
			continue
		}
		pool.Address = account.ID.ToRaw()
		b.tfPools[account.ID] = pool
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

func (b *Book) getGGWhitelist() error {
	addresses, err := fetchGetGemsVerifiedCollections()
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, account := range addresses {
		collection, ok := b.collections[account]
		if !ok {
			collection.Address = account.ToRaw()
			collection.Approvers = []oas.NftApprovedByItem{oas.NftApprovedByItemGetgems}
			b.collections[account] = collection
			continue
		}
		if !slices.Contains(collection.Approvers, oas.NftApprovedByItemGetgems) {
			collection.Approvers = append(collection.Approvers, oas.NftApprovedByItemGetgems)
			b.collections[account] = collection
		}
	}
	return nil
}

func fetchGetGemsVerifiedCollections() ([]tongo.AccountID, error) {
	client := graphql.NewClient("https://api.getgems.io/graphql", nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var q struct {
		GetAddressesOfVerifiedCollections []graphql.String `graphql:"getAddressesOfVerifiedCollections"`
	}
	err := client.Query(ctx, &q, nil)
	if err != nil {
		return nil, err
	}
	accountIDs := make([]tongo.AccountID, 0, len(q.GetAddressesOfVerifiedCollections))
	for _, collection := range q.GetAddressesOfVerifiedCollections {
		account, err := tongo.ParseAddress(string(collection))
		if err != nil {
			return nil, err
		}
		accountIDs = append(accountIDs, account.ID)
	}
	return accountIDs, nil
}
