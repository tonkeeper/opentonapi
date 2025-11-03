package bath

import (
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
)

func parseAccount(a tlb.MsgAddress) *Account {
	o, err := tongo.AccountIDFromTlb(a)
	if err == nil && o != nil {
		return &Account{Address: *o}
	}
	return nil
}

type Merge struct {
	children []*Bubble
}
