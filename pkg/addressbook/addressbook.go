package addressbook

import (
	"encoding/json"
	"fmt"
	"golang.org/x/exp/maps"
	"io"
	"net/http"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"
)

// KnownAddress represents additional manually crafted information about a particular account in the blockchain.
type KnownAddress struct {
	IsScam      bool `json:"is_scam,omitempty"`
	RequireMemo bool `json:"require_memo,omitempty"`
	// Name is a dns name.
	Name    string `json:"name"`
	Address string `json:"address"`
	Image   string `json:"image,omitempty"`
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
}

// Book holds information about known accounts, jettons, NFT collections manually crafted by the tonkeeper team and the community.
type Book struct {
	addresses   map[tongo.AccountID]KnownAddress
	collections map[tongo.AccountID]KnownCollection
	jettons     map[tongo.AccountID]KnownJetton
	tfPools     map[tongo.AccountID]TFPoolInfo
	dnsCache    map[tongo.AccountID]string
}

type TFPoolInfo struct {
	Name      string `json:"name"`
	GroupName string `json:"groupName"`
	Address   string `json:"address"`
}

func (b *Book) GetAddressInfoByAddress(a tongo.AccountID) (KnownAddress, bool) {
	a1, ok := b.addresses[a]
	if ok {
		return a1, ok
	}
	name, ok := b.dnsCache[a]
	a1.Name = name
	a1.Address = a.ToRaw()
	return a1, ok
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

func NewAddressBook(logger *zap.Logger, addressPath, jettonPath, collectionPath string) *Book {
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
			collections[a] = i
		}
	}
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
		dnsCache:    map[tongo.AccountID]string{},
		tfPools:     tfPools,
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
