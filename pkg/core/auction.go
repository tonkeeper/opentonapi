package core

import "github.com/tonkeeper/tongo"

type Auction struct {
	Bids   int64
	Date   int64
	Domain string
	Owner  tongo.AccountID
	Price  int64
}

type DomainBid struct {
	Bidder  tongo.AccountID
	Success bool
	TxTime  int64
	Value   uint64
	TxHash  tongo.Bits256
}
