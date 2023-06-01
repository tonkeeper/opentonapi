package bath

import (
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"golang.org/x/exp/slices"
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
	return slices.Contains(a.Interfaces, i)
}
