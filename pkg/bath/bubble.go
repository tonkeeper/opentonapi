package bath

import "github.com/tonkeeper/tongo"

type Bubble struct {
	TypeName string
	Info     any
	Children []Bubble
	Fee      Fee
}

type Fee struct {
	WhoPay  tongo.AccountID
	Compute uint64
	Storage uint64
	Deposit uint64
}
