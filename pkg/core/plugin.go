package core

import (
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

type Plugin struct {
	AccountID ton.AccountID
	Status    tlb.AccountStatus
	Type      string
}
