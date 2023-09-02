package bath

import (
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

type BubbleContractDeploy struct {
	Contract tongo.AccountID
	// AccountInitInterfaces is a list of interfaces implemented by the stateInit.Code.
	// This list can differ from the current list of interfaces.
	// TODO: AccountInitInterfaces is an empty list in opentonapi, fix.
	AccountInitInterfaces []abi.ContractInterface
	Success               bool
}

func (b BubbleContractDeploy) ToAction() *Action {
	return &Action{
		ContractDeploy: &ContractDeployAction{
			Address:    b.Contract,
			Interfaces: b.AccountInitInterfaces,
		},
		Success: b.Success,
		Type:    ContractDeploy,
	}
}
