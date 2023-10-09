package bath

import (
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

type Account struct {
	Address    tongo.AccountID
	Interfaces []abi.ContractInterface
}

func (a *Account) Addr() *tongo.AccountID {
	if a == nil {
		return nil
	}
	return &a.Address
}

func (a Account) Is(i abi.ContractInterface) bool {
	for _, iface := range a.Interfaces {
		if iface.Implements(i) {
			return true
		}
	}
	return false
}
