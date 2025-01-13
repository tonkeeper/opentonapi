package addressbook

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"github.com/tonkeeper/opentonapi/pkg/cache"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

// NormalizeReg is a regular expression to remove all non-letter and non-number characters
var NormalizeReg = regexp.MustCompile("[^\\p{L}\\p{N}]")

// KnownAddress represents additional manually crafted information about a particular account in the blockchain
type KnownAddress struct {
	IsScam      bool   `json:"is_scam,omitempty"`
	RequireMemo bool   `json:"require_memo,omitempty"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	Image       string `json:"image,omitempty"`
}

// AttachedAccountType defines different types of accounts (e.g., manual, NFT)
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

// KnownJetton represents additional manually crafted information about a particular jetton in the blockchain
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

// KnownCollection represents additional manually crafted information about a particular NFT collection in the blockchain
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

// Book holds information about known accounts, jettons, NFT collections manually crafted by the tonkeeper team and the community
type Book struct {
	addressers []addresser

	states          accountsStatesSource
	mu              sync.RWMutex
	collections     map[tongo.AccountID]KnownCollection
	jettons         map[tongo.AccountID]KnownJetton
	tfPools         map[tongo.AccountID]TFPoolInfo
	walletsResolved cache.Cache[tongo.AccountID, bool]
}

// TFPoolInfo holds information about a token pool
type TFPoolInfo struct {
	Name      string `json:"name"`
	GroupName string `json:"groupName"`
	Address   string `json:"address"`
}

// GetAddressInfoByAddress fetches address info if available
func (b *Book) GetAddressInfoByAddress(a tongo.AccountID) (KnownAddress, bool) {
	for i := range b.addressers {
		if a1, ok := b.addressers[i].GetAddress(a); ok {
			return a1, ok
		}
	}
	return KnownAddress{}, false
}

// SearchAttachedAccountsByPrefix searches for accounts by prefix
func (b *Book) SearchAttachedAccountsByPrefix(prefix string) []AttachedAccount {
	if prefix == "" {
		return []AttachedAccount{}
	}
	prefix = strings.ToLower(NormalizeReg.ReplaceAllString(prefix, ""))
	exclusiveAccounts := make(map[string]AttachedAccount)
	for i := range b.addressers {
		foundAccounts := b.addressers[i].SearchAttachedAccounts(prefix)
		for _, account := range foundAccounts {
			key := fmt.Sprintf("%v:%v", account.Wallet.ToRaw(), account.Normalized)
			existing, ok := exclusiveAccounts[key]
			// Ensure only one account per wallet is included
			if !ok || (existing.Trust != core.TrustWhitelist && account.Trust == core.TrustWhitelist) {
				exclusiveAccounts[key] = account
			}
		}
	}
	accounts := maps.Values(exclusiveAccounts)
	tonDomainPrefix := prefix + "ton"
	tgDomainPrefix := prefix + "tme"
	// Boost weight for accounts that match the prefix
	for i := range accounts {
		normalized := accounts[i].Normalized
		if normalized == prefix || normalized == tonDomainPrefix || normalized == tgDomainPrefix {
			accounts[i].Weight *= BoostForFullMatch
		}
		if accounts[i].Trust == core.TrustWhitelist {
			accounts[i].Weight *= BoostForVerified
		}
	}
	// Sort accounts by weight and name length
	sort.Slice(accounts, func(i, j int) bool {
		if accounts[i].Weight == accounts[j].Weight {
			return len(accounts[i].Name) < len(accounts[j].Name)
		}
		return accounts[i].Weight > accounts[j].Weight
	})
	// Limit the result to 50 accounts
	if len(accounts) > 50 {
		accounts = accounts[:50]
	}
	return accounts
}

// GetTFPoolInfo retrieves token pool info for an account
func (b *Book) GetTFPoolInfo(a tongo.AccountID) (TFPoolInfo, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	info, ok := b.tfPools[a]
	return info, ok
}

// GetKnownCollections returns all known collections
func (b *Book) GetKnownCollections() map[tongo.AccountID]KnownCollection {
	b.mu.RLock()
	defer b.mu.RUnlock()

	collections := make(map[tongo.AccountID]KnownCollection, len(b.collections))
	for accountID, collection := range b.collections {
		collections[accountID] = collection
	}
	return collections
}

// GetCollectionInfoByAddress retrieves collection info for a specific address
func (b *Book) GetCollectionInfoByAddress(a tongo.AccountID) (KnownCollection, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	c, ok := b.collections[a]
	return c, ok
}

// GetKnownJettons returns all known jettons
func (b *Book) GetKnownJettons() map[tongo.AccountID]KnownJetton {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return maps.Clone(b.jettons)
}

// GetJettonInfoByAddress fetches jetton info for a specific address
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

// TFPools returns a list of all token pools
func (b *Book) TFPools() []tongo.AccountID {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return maps.Keys(b.tfPools)
}

// IsWallet checks if the address is a wallet
func (b *Book) IsWallet(addr tongo.AccountID) (bool, error) {
	if wallet, ok := b.walletsResolved.Get(addr); ok {
		return wallet, nil
	}
	status, interfaces, err := b.states.AccountStatusAndInterfaces(addr)
	if err != nil {
		return false, fmt.Errorf("failed to figure out if %v is a wallet %w", addr, err)
	}
	if status == tlb.AccountNone || status == tlb.AccountUninit {
		b.walletsResolved.Set(addr, true, cache.WithExpiration(time.Minute)) // Cache short for non-existing accounts
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

type manualAddresser struct {
	mu        sync.RWMutex
	addresses map[tongo.AccountID]KnownAddress
	sorted    []AttachedAccount
}

// GetAddress fetches known address by account
func (m *manualAddresser) GetAddress(a tongo.AccountID) (KnownAddress, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	addr, ok := m.addresses[a]
	return addr, ok
}

// SearchAttachedAccounts searches for accounts by prefix
func (m *manualAddresser) SearchAttachedAccounts(prefix string) []AttachedAccount {
	m.mu.RLock()
	sortedList := m.sorted
	m.mu.RUnlock()

	if len(sortedList) == 0 {
		return []AttachedAccount{}
	}
	// Normalize the prefix for comparison
	prefix = strings.ToLower(NormalizeReg.ReplaceAllString(prefix, ""))
	startIdx, endIdx := FindIndexes(sortedList, prefix)
	if startIdx == -1 || endIdx == -1 {
		return []AttachedAccount{}
	}
	// Return the found accounts as a slice
	foundAccounts := make([]AttachedAccount, endIdx-startIdx+1)
	copy(foundAccounts, sortedList[startIdx:endIdx+1])
	return foundAccounts
}

// refreshAddresses updates the list of known addresses
func (m *manualAddresser) refreshAddresses(addressPath string) error {
	addresses, err := downloadJson[KnownAddress](addressPath)
	if err != nil {
		return err
	}
	knownAccounts := make(map[tongo.AccountID]KnownAddress, len(addresses))
	attachedAccounts := make([]AttachedAccount, 0, len(addresses)*3)
	for _, item := range addresses {
		account, err := tongo.ParseAddress(item.Address)
		if err != nil {
			continue
		}
		item.Address = account.ID.ToRaw()
		knownAccounts[account.ID] = item
		// Generate name variants for the account
		names := GenerateNameVariants(item.Name)
		for idx, name := range names {
			weight := KnownAccountWeight
			if idx == 0 { // Boost weight for the first name
				weight *= BoostForOriginalName
			}
			// Convert known account to attached account
			attachedAccount, err := ConvertAttachedAccount(name, item.Image, account.ID, weight, core.TrustWhitelist, ManualAccountType)
			if err != nil {
				continue
			}
			attachedAccounts = append(attachedAccounts, attachedAccount)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.addresses = knownAccounts
	sorted := m.sorted
	sorted = append(sorted, attachedAccounts...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Normalized < sorted[j].Normalized
	})
	m.sorted = sorted

	return nil
}

// NewAddressBook initializes a Book and starts background refreshers tasks
func NewAddressBook(logger *zap.Logger, addressPath, jettonPath, collectionPath string, storage accountsStatesSource, opts ...Option) *Book {
	var manual = &manualAddresser{
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
	// Start background refreshers
	go Refresher("gg whitelist", time.Hour, 5*time.Minute, logger, book.getGGWhitelist)
	go Refresher("addresses", time.Minute*15, 5*time.Minute, logger, func() error { return manual.refreshAddresses(addressPath) })
	go Refresher("jettons", time.Minute*15, 5*time.Minute, logger, func() error { return book.refreshJettons(manual, jettonPath) })
	go Refresher("collections", time.Minute*15, 5*time.Minute, logger, func() error { return book.refreshCollections(collectionPath) })
	book.refreshTfPools(logger) // Refresh tfPools once on initialization as it doesn't need periodic updates

	return book
}

// Refresher periodically calls the provided function at the specified interval
func Refresher(name string, interval, errorInterval time.Duration, logger *zap.Logger, f func() error) {
	for {
		err := f()
		if err != nil {
			logger.Error("refresh "+name, zap.Error(err))
			time.Sleep(errorInterval + time.Duration(rand.Intn(10))*time.Second)
			continue
		}
		// Wait for the next interval before refreshing again
		time.Sleep(interval + time.Duration(rand.Intn(10))*time.Second)
	}
}

// refreshJettons fetches and updates the jetton data from the provided URL
func (b *Book) refreshJettons(addresser *manualAddresser, jettonPath string) error {
	// Download jettons data
	jettons, err := downloadJson[KnownJetton](jettonPath)
	if err != nil {
		return err
	}
	knownJettons := make(map[tongo.AccountID]KnownJetton, len(jettons))
	var attachedAccounts []AttachedAccount
	for _, item := range jettons {
		account, err := tongo.ParseAddress(item.Address)
		if err != nil {
			continue
		}
		item.Address = account.ID.ToRaw()
		knownJettons[account.ID] = item
		// Generate name variants for the jetton
		names := GenerateNameVariants(item.Name)
		for idx, name := range names {
			weight := KnownAccountWeight
			if idx == 0 { // Boost weight for the first name
				weight *= BoostForOriginalName
			}
			// Convert known account to attached account
			attachedAccount, err := ConvertAttachedAccount(name, item.Image, account.ID, weight, core.TrustWhitelist, JettonNameAccountType)
			if err != nil {
				continue
			}
			attachedAccounts = append(attachedAccounts, attachedAccount)
		}
	}

	b.mu.Lock()
	b.jettons = knownJettons
	b.mu.Unlock()

	addresser.mu.Lock()
	defer addresser.mu.Unlock()
	sorted := addresser.sorted
	sorted = append(sorted, attachedAccounts...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Normalized < sorted[j].Normalized
	})
	addresser.sorted = sorted

	return nil
}

// refreshCollections fetches and updates collection data from the provided URL
func (b *Book) refreshCollections(collectionPath string) error {
	collections, err := downloadJson[KnownCollection](collectionPath)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	// Update collections map with the fetched data
	for _, item := range collections {
		// TODO: remove items that were previously added but aren't present in the current list
		account, err := tongo.ParseAddress(item.Address)
		if err != nil {
			continue
		}
		currentCollection, ok := b.collections[account.ID]
		if !ok {
			// Add new collection with Tonkeeper as the approver
			currentCollection.Address = account.ID.ToRaw()
			currentCollection.Approvers = []oas.NftApprovedByItem{oas.NftApprovedByItemTonkeeper}
			b.collections[account.ID] = currentCollection
			continue
		}
		// Merge approvers and ensure Tonkeeper is included
		if !slices.Contains(currentCollection.Approvers, oas.NftApprovedByItemTonkeeper) {
			currentCollection.Approvers = append(item.Approvers, oas.NftApprovedByItemTonkeeper)
			b.collections[account.ID] = currentCollection
		}
	}
	return nil
}

// refreshTfPools fetches and updates the TF pool data
func (b *Book) refreshTfPools(logger *zap.Logger) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// Fetch and update the TF pools
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

// downloadJson is a utility function that downloads and unmarshals JSON data from a URL into a slice of the specified type
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

// getGGWhitelist fetches the whitelist from the GetGems API and updates the collections
func (b *Book) getGGWhitelist() error {
	addresses, err := fetchGetGemsVerifiedCollections()
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	// Update collections with new addresses from the whitelist
	for _, account := range addresses {
		collection, ok := b.collections[account]
		if !ok {
			// Add new collection with Getgems as the approver
			collection.Address = account.ToRaw()
			collection.Approvers = []oas.NftApprovedByItem{oas.NftApprovedByItemGetgems}
			b.collections[account] = collection
			continue
		}
		// Ensure Getgems is included as an approver
		if !slices.Contains(collection.Approvers, oas.NftApprovedByItemGetgems) {
			collection.Approvers = append(collection.Approvers, oas.NftApprovedByItemGetgems)
			b.collections[account] = collection
		}
	}
	return nil
}

// fetchGetGemsVerifiedCollections fetches verified collections from GetGems API
func fetchGetGemsVerifiedCollections() ([]tongo.AccountID, error) {
	res, err := downloadJson[string]("https://api.getgems.io/public/api/verified-collections")
	if err != nil {
		return nil, err
	}
	accountIDs := make([]tongo.AccountID, 0, len(res))
	for _, collection := range res {
		account, err := tongo.ParseAddress(collection)
		if err != nil {
			return nil, err
		}
		accountIDs = append(accountIDs, account.ID)
	}
	return accountIDs, nil
}
