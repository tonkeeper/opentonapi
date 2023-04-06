package core

import (
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
	Metadata          []byte
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
