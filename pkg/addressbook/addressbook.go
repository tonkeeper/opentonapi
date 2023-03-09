package addressbook

import (
	"encoding/json"
	"fmt"
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
	addresses   map[string]KnownAddress
	collections map[string]KnownCollection
	jettons     map[string]KnownJetton
	dnsCache    map[string]string
}

func (b *Book) GetAddressInfoByAddress(rawAddr string) (KnownAddress, bool) {
	a1, ok := b.addresses[rawAddr]
	if ok {
		return a1, ok
	}
	name, ok := b.dnsCache[rawAddr]
	a1.Name = name
	a1.Address = rawAddr
	return a1, ok
}

func (b *Book) GetCollectionInfoByAddress(rawAddr string) (KnownCollection, bool) {
	c, ok := b.collections[rawAddr]
	return c, ok
}

func (b *Book) GetJettonInfoByAddress(rawAddr string) (KnownJetton, bool) {
	j, ok := b.jettons[rawAddr]
	if ok {
		j.Verification = Whitelist
	} else {
		j.Verification = None
	}
	return j, ok
}

func NewAddressBook(logger *zap.Logger, addressPath, jettonPath, collectionPath string) *Book {
	addresses := make(map[string]KnownAddress)
	collections := make(map[string]KnownCollection)
	jettons := make(map[string]KnownJetton)

	addrs, err := downloadJson[KnownAddress](addressPath)
	if err != nil {
		logger.Info("fail to load accounts.json")
	} else {
		for _, i := range addrs {
			a, err := convertAddressToRaw(i.Address)
			if err != nil {
				continue
			}
			i.Address = a
			addresses[a] = i
		}
	}
	jetts, err := downloadJson[KnownJetton](jettonPath)
	if err != nil {
		logger.Info("fail to load jettons.json")
	} else {
		for _, i := range jetts {
			a, err := convertAddressToRaw(i.Address)
			if err != nil {
				continue
			}
			i.Address = a
			jettons[i.Address] = i
		}
	}
	colls, err := downloadJson[KnownCollection](collectionPath)
	if err != nil {
		logger.Info("fail to load collections.json")
	} else {
		for _, i := range colls {
			a, err := convertAddressToRaw(i.Address)
			if err != nil {
				continue
			}
			i.Address = a
			collections[i.Address] = i
		}
	}
	return &Book{
		addresses:   addresses,
		collections: collections,
		jettons:     jettons,
		dnsCache:    map[string]string{},
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

func convertAddressToRaw(a string) (string, error) {
	addr, err := tongo.ParseAccountID(a)
	if err != nil {
		return "", err
	}
	return addr.ToRaw(), nil
}
