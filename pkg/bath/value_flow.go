package bath

import (
	"math/big"

	"github.com/tonkeeper/tongo"
)

// AccountValueFlow contains a change of assets for a particular account.
type AccountValueFlow struct {
	Ton     int64
	Fees    int64
	Jettons map[tongo.AccountID]big.Int
}

// ValueFlow contains a change of assets for each account involved in a trace.
type ValueFlow struct {
	Accounts map[tongo.AccountID]*AccountValueFlow
}

func newValueFlow() *ValueFlow {
	return &ValueFlow{
		Accounts: map[tongo.AccountID]*AccountValueFlow{},
	}
}

func (flow *ValueFlow) AddTons(accountID tongo.AccountID, amount int64) {
	if accountFlow, ok := flow.Accounts[accountID]; ok {
		accountFlow.Ton += amount
		return
	}
	flow.Accounts[accountID] = &AccountValueFlow{Ton: amount}
}

func (flow *ValueFlow) AddFee(accountID tongo.AccountID, amount int64) {
	if accountFlow, ok := flow.Accounts[accountID]; ok {
		accountFlow.Fees += amount
		return
	}
	flow.Accounts[accountID] = &AccountValueFlow{Fees: amount}
}

func (flow *ValueFlow) AddJettons(accountID tongo.AccountID, jetton tongo.AccountID, value big.Int) {
	accountFlow, ok := flow.Accounts[accountID]
	if !ok {
		accountFlow = &AccountValueFlow{}
		flow.Accounts[accountID] = accountFlow
	}
	if accountFlow.Jettons == nil {
		accountFlow.Jettons = make(map[tongo.AccountID]big.Int, 1)
	}
	current := accountFlow.Jettons[jetton]
	newValue := big.Int{}
	newValue.Add(&current, &value)
	accountFlow.Jettons[jetton] = newValue
}

func (flow *ValueFlow) Merge(other *ValueFlow) {
	for accountID, af := range other.Accounts {
		flow.AddTons(accountID, af.Ton)
		flow.AddFee(accountID, af.Fees)
		for jetton, value := range af.Jettons {
			flow.AddJettons(accountID, jetton, value)
		}
	}
}
