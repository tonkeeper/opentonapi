package bath

import (
	"math/big"

	"github.com/tonkeeper/tongo"
)

var (
	zero = big.NewInt(0)
)

// AccountValueFlow contains a change of assets for a particular account.
type AccountValueFlow struct {
	Ton     int64
	Fees    int64
	Jettons map[tongo.AccountID]big.Int
	NFTs    [2]int // 0 - added, 1 - removed
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
		accountFlow.Ton -= amount
		return
	}
	flow.Accounts[accountID] = &AccountValueFlow{Fees: amount, Ton: -amount}
}
func (flow *ValueFlow) SubJettons(accountID tongo.AccountID, jettonMaster tongo.AccountID, value big.Int) {
	var negative big.Int
	negative.Neg(&value)
	flow.AddJettons(accountID, jettonMaster, negative)
}

func (flow *ValueFlow) AddJettons(accountID tongo.AccountID, jettonMaster tongo.AccountID, value big.Int) {
	accountFlow, ok := flow.Accounts[accountID]
	if !ok {
		accountFlow = &AccountValueFlow{}
		flow.Accounts[accountID] = accountFlow
	}
	if accountFlow.Jettons == nil {
		accountFlow.Jettons = make(map[tongo.AccountID]big.Int, 1)
	}
	current := accountFlow.Jettons[jettonMaster]
	newValue := big.Int{}
	newValue.Add(&current, &value)
	accountFlow.Jettons[jettonMaster] = newValue
	if newValue.Cmp(zero) == 0 {
		delete(accountFlow.Jettons, jettonMaster)
	}
}

func (flow *ValueFlow) Merge(other *ValueFlow) {
	for accountID, af := range other.Accounts {
		if _, ok := flow.Accounts[accountID]; !ok {
			flow.Accounts[accountID] = &AccountValueFlow{}
		}
		flow.Accounts[accountID].Ton += af.Ton
		flow.Accounts[accountID].Fees += af.Fees
		flow.Accounts[accountID].NFTs[0] += af.NFTs[0]
		flow.Accounts[accountID].NFTs[1] += af.NFTs[1]
		for jetton, value := range af.Jettons {
			flow.AddJettons(accountID, jetton, value)
		}
	}
}
