package bath

import (
	"sort"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"golang.org/x/exp/slices"
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

func ProcessChildren(children []*Bubble, fns ...func(child *Bubble) *Merge) []*Bubble {
	var newChildren []*Bubble
	for _, child := range children {
		merged := false
		for _, fn := range fns {
			merge := fn(child)
			if merge != nil {
				newChildren = append(newChildren, merge.children...)
				merged = true
				break
			}
		}
		if !merged {
			newChildren = append(newChildren, child)
		}
	}
	return newChildren
}

func noOpCode(bubble *Bubble) bool {
	txBubble, ok := bubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	return txBubble.opCode == nil
}

func operation(bubble *Bubble, opName abi.MsgOpName) bool {
	txBubble, ok := bubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	return txBubble.operation(opName)
}

func distinctAccounts(accounts ...tongo.AccountID) []tongo.AccountID {
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].String() < accounts[j].String()
	})
	return slices.Compact(accounts)
}
