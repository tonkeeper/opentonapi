package core

import "github.com/tonkeeper/tongo/abi"

type Trace struct {
	// Transaction is slightly modified.
	// For example, we have kept only external outbound messages in OutMsgs.
	Transaction
	AccountInterfaces []abi.ContractInterface
	Children          []*Trace
	AdditionalInfo    map[string]interface{}
}

func (t *Trace) InProgress() bool {
	return t.countUncompleted() != 0
}
func (t *Trace) countUncompleted() int {
	c := len(t.OutMsgs) //todo: not count externals
	for _, st := range t.Children {
		c += st.countUncompleted()
	}
	return c
}
