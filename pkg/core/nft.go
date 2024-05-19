package core

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/tongo"
)

type NftItem struct {
	Address           tongo.AccountID
	Index             decimal.Decimal
	CollectionAddress *tongo.AccountID
	OwnerAddress      *tongo.AccountID
	Verified          bool
	Transferable      bool
	DNS               *string
	Sale              *NftSaleInfo
	Metadata          map[string]interface{}
}

type NftCollection struct {
	Address           tongo.AccountID
	NextItemIndex     uint64
	OwnerAddress      *tongo.AccountID
	ContentLayout     int
	CollectionContent []byte
	InWhitelist       bool
	Metadata          map[string]interface{}
}

type NftSaleInfo struct {
	Contract    tongo.AccountID
	Marketplace tongo.AccountID
	Nft         tongo.AccountID
	Seller      *tongo.AccountID
	Price       struct {
		Token  *tongo.AccountID
		Amount uint64
	}
	MarketplaceFee uint64
	RoyaltyAddress *tongo.AccountID
	RoyaltyAmount  uint64
}

func GetNftMetaData(nftMetaUrl string) ([]byte, error) {
	if !strings.HasPrefix(nftMetaUrl, "http://") && !strings.HasPrefix(nftMetaUrl, "https://") {
		return nil, fmt.Errorf("not http/https link")
	}
	var client = &http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(nftMetaUrl)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return nil, fmt.Errorf("invalid status code: %v", response.StatusCode)
	}
	meta, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return meta, nil
}
