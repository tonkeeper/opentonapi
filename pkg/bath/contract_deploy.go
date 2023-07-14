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

// FindContractDeploy looks for contract deployments in the given bubble and
// creates a new ContractDeploy action for each.
// So at the end the bubble will look like:
// bubble -> ContractDeploy -> ContractDeploy -> bubble.children.
func FindContractDeploy(bubble *Bubble) bool {
	if len(bubble.ContractDeployments) == 0 {
		return false
	}
	for accountID, deployment := range bubble.ContractDeployments {
		newBubble := Bubble{
			Info: BubbleContractDeploy{
				Contract:              accountID,
				AccountInitInterfaces: deployment.initInterfaces,
				Success:               deployment.success,
			},
			Accounts:  []tongo.AccountID{accountID},
			Children:  bubble.Children,
			ValueFlow: &ValueFlow{},
		}
		bubble.Children = []*Bubble{&newBubble}
	}
	bubble.ContractDeployments = nil
	return true
}
