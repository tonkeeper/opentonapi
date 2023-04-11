package core

import "github.com/tonkeeper/tongo"

type Auction struct {
	Bids   int64
	Date   int64
	Domain string
	Owner  tongo.AccountID
	Price  int64
}
