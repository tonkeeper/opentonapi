package addressbook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/exp/maps"

	"github.com/shopspring/decimal"
	"github.com/shurcooL/graphql"
	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"
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
	Wallet     string `json:"wallet"`
	Normalized string
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
	Approvers   []string
}

type Options struct {
	addressers []addresser
}
type Option func(o *Options)

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
	for i := range b.addressers {
		foundAccounts := b.addressers[i].SearchAttachedAccounts(prefix)
		if len(foundAccounts) > 0 {
			return foundAccounts
		}
	}
	return []AttachedAccount{}
}

func (b *Book) GetTFPoolInfo(a tongo.AccountID) (TFPoolInfo, bool) {
	info, ok := b.tfPools[a]
	return info, ok
}

func (b *Book) GetCollectionInfoByAddress(a tongo.AccountID) (KnownCollection, bool) {
	c, ok := b.collections[a]
	return c, ok
}

func (b *Book) GetJettonInfoByAddress(a tongo.AccountID) (KnownJetton, bool) {
	j, ok := b.jettons[a]
	if ok {
		j.Verification = Whitelist
	} else {
		j.Verification = None
	}
	return j, ok
}

func (b *Book) GetKnownJettons() map[tongo.AccountID]KnownJetton {
	return b.jettons
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

	addrs, err := downloadJson[KnownAddress](addressPath)
	if err != nil {
		logger.Info("fail to load accounts.json")
	} else {
		for _, i := range addrs {
			a, err := tongo.ParseAccountID(i.Address)
			if err != nil {
				continue
			}
			i.Address = a.ToRaw()

			addresses[a] = i
		}
	}
	jetts, err := downloadJson[KnownJetton](jettonPath)
	if err != nil {
		logger.Info("fail to load jettons.json")
	} else {
		for _, i := range jetts {
			a, err := tongo.ParseAccountID(i.Address)
			if err != nil {
				continue
			}
			i.Address = a.ToRaw()
			jettons[a] = i
		}
	}
	redoubtJetts, err := getRedoubtJettons()
	if err != nil {
		logger.Info("fail to load redoubt jettons")
	} else {
		for _, jetton := range redoubtJetts {
			_, ok := jettons[tongo.MustParseAccountID(jetton.Address)]
			if !ok {
				jettons[tongo.MustParseAccountID(jetton.Address)] = jetton
			}
		}
	}
	stonfiJettons, err := getStonfiJettons()
	if err != nil {
		logger.Info("fail to load stonfi jettons")
	} else {
		for _, jetton := range stonfiJettons {
			_, ok := jettons[tongo.MustParseAccountID(jetton.Address)]
			if !ok {
				jettons[tongo.MustParseAccountID(jetton.Address)] = jetton
			}
		}
	}
	colls, err := downloadJson[KnownCollection](collectionPath)
	if err != nil {
		logger.Info("fail to load collections.json")
	} else {
		for _, i := range colls {
			a, err := tongo.ParseAccountID(i.Address)
			if err != nil {
				continue
			}
			i.Address = a.ToRaw()
			i.Approvers = append(i.Approvers, "tonkeeper")
			collections[a] = i
		}
	}
	go getGGWhitelist(collections, logger)
	for _, v := range getPools(logger) {
		a, err := tongo.ParseAccountID(v.Address)
		if err != nil {
			logger.Error("parse account in pools", zap.Error(err))
			continue
		}
		v.Address = a.ToRaw()
		tfPools[a] = v
	}

	return &Book{
		addresses:   addresses,
		collections: collections,
		jettons:     jettons,
		tfPools:     tfPools,
		addressers:  options.addressers,
	}
}

func (b *Book) TFPools() []tongo.AccountID {
	return maps.Keys(b.tfPools)
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

func getGGWhitelist(collections map[tongo.AccountID]KnownCollection, logger *zap.Logger) {
	client := graphql.NewClient("https://api.getgems.io/graphql", nil)
	for {
		addresses, err := _getGGWhitelist(client)
		if err != nil {
			logger.Info(fmt.Sprintf("get nft collection whitelist: %v", err))
			time.Sleep(time.Minute * 3)
			continue
		}
		for _, a := range addresses {
			c := collections[a]
			c.Approvers = append(c.Approvers, "getgems")
			collections[a] = c
		}
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

func getRedoubtJettons() ([]KnownJetton, error) {
	response, err := http.Get("https://api.redoubt.online/v2/feed/jettons")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return nil, fmt.Errorf("invalid status code %v", response.StatusCode)
	}
	var respBody struct {
		Jettons []KnownJetton `json:"jettons"`
	}
	if err = json.NewDecoder(response.Body).Decode(&respBody); err != nil {
		return nil, err
	}
	for idx, jetton := range respBody.Jettons {
		address, err := tongo.ParseAccountID(jetton.Address)
		if err != nil {
			continue
		}
		jetton.Address = address.ToRaw()
		respBody.Jettons[idx] = jetton
	}
	return respBody.Jettons, nil
}

func getStonfiJettons() ([]KnownJetton, error) {
	response, err := http.Get("https://api.ston.fi/v1/pools")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return nil, fmt.Errorf("invalid status code %v", response.StatusCode)
	}
	var respBody struct {
		PoolList []struct {
			Address string `json:"address"`
		} `json:"pool_list"`
	}
	if err = json.NewDecoder(response.Body).Decode(&respBody); err != nil {
		return nil, err
	}
	var jettons []KnownJetton
	for _, jetton := range respBody.PoolList {
		address, err := tongo.ParseAccountID(jetton.Address)
		if err != nil {
			continue
		}
		jettons = append(jettons, KnownJetton{Address: address.ToRaw()})
	}
	return jettons, nil
}
