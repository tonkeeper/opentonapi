package bath

import (
	"cmp"
	"encoding/binary"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
	"golang.org/x/exp/slices"
)

func CollectActionsAndValueFlow(bubble *Bubble, forAccount *tongo.AccountID) ([]Action, *ValueFlow) {
	var actions []Action
	valueFlow := newValueFlow()
	if forAccount == nil || slices.Contains(bubble.Accounts, *forAccount) {
		a := bubble.Info.ToAction()
		if a != nil {
			slices.SortFunc(bubble.Transaction, func(a, b ton.Bits256) int {
				return cmp.Compare(binary.NativeEndian.Uint64(a[:8]), binary.NativeEndian.Uint64(b[:8])) //just for compact
			})
			a.BaseTransactions = slices.Compact(bubble.Transaction)
			actions = append(actions, *a)
		}
	}
	for _, c := range bubble.Children {
		childActions, childValueFlow := CollectActionsAndValueFlow(c, forAccount)
		actions = append(actions, childActions...)
		valueFlow.Merge(childValueFlow)
	}
	valueFlow.Merge(bubble.ValueFlow)
	return actions, valueFlow
}
