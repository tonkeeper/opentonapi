package bath

import (
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
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
