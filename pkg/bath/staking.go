package bath

import (
	"github.com/tonkeeper/opentonapi/pkg/blockchain/config"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

type BubbleDepositStake struct {
	Staker  tongo.AccountID
	Amount  int64
	Success bool
}

func (ds BubbleDepositStake) ToAction() *Action {
	return &Action{
		DepositStake: &DepositStakeAction{
			Amount:  ds.Amount,
			Elector: config.ElectorAddress(),
			Staker:  ds.Staker,
		},
		Success: ds.Success,
		Type:    DepositStake,
	}
}

func FindDepositStake(bubble *Bubble) bool {
	bubbleTx, ok := bubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	if !bubbleTx.operation(abi.ElectorNewStakeMsgOp) {
		return false
	}
	if bubbleTx.account.Address != config.ElectorAddress() {
		return false
	}
	stake := BubbleDepositStake{
		Amount: bubbleTx.inputAmount,
		Staker: bubbleTx.inputFrom.Address,
	}
	newBubble := Bubble{
		Accounts:            bubble.Accounts,
		ValueFlow:           bubble.ValueFlow,
		ContractDeployments: bubble.ContractDeployments,
	}
	newBubble.Children = ProcessChildren(bubble.Children,
		func(child *Bubble) *Merge {
			confirmation, ok := child.Info.(BubbleTx)
			if !ok {
				return nil
			}
			if !confirmation.operation(abi.ElectorNewStakeConfirmationMsgOp) {
				return nil
			}
			stake.Success = true
			newBubble.ValueFlow.Merge(child.ValueFlow)
			newBubble.MergeContractDeployments(child)
			return &Merge{children: child.Children}
		})
	newBubble.Info = stake
	*bubble = newBubble
	return true
}

type BubbleRecoverStake struct {
	Staker  tongo.AccountID
	Amount  int64
	Success bool
}

func (b BubbleRecoverStake) ToAction() *Action {
	return &Action{
		RecoverStake: &RecoverStakeAction{
			Amount:  b.Amount,
			Elector: config.ElectorAddress(),
			Staker:  b.Staker,
		},
		Success: b.Success,
		Type:    RecoverStake,
	}
}

func FindRecoverStake(bubble *Bubble) bool {
	bubbleTx, ok := bubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	if !bubbleTx.operation(abi.ElectorRecoverStakeRequestMsgOp) {
		return false
	}
	if bubbleTx.account.Address != config.ElectorAddress() {
		return false
	}
	recoverStake := BubbleRecoverStake{
		Amount: 0,
		Staker: bubbleTx.inputFrom.Address,
	}
	newBubble := Bubble{
		Accounts:            bubble.Accounts,
		ValueFlow:           bubble.ValueFlow,
		ContractDeployments: bubble.ContractDeployments,
	}
	newBubble.Children = ProcessChildren(bubble.Children,
		func(child *Bubble) *Merge {
			response, ok := child.Info.(BubbleTx)
			if !ok {
				return nil
			}
			if !response.operation(abi.ElectorRecoverStakeResponseMsgOp) {
				return nil
			}
			recoverStake.Success = true
			recoverStake.Amount = response.inputAmount
			newBubble.ValueFlow.Merge(child.ValueFlow)
			newBubble.MergeContractDeployments(child)
			return &Merge{children: child.Children}
		})
	newBubble.Info = recoverStake
	*bubble = newBubble
	return true

}
