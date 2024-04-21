package bath

import (
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

type TFCommand string

// todo: move to tongo
const (
	TfDepositStakeRequest            TFCommand = "TfDepositStakeRequest"
	TfRecoverStakeRequest            TFCommand = "TfRecoverStakeRequest"
	TfProcessPendingWithdrawRequests TFCommand = "TfProcessPendingWithdrawRequests"
	TfUpdateValidatorSet             TFCommand = "TfUpdateValidatorSet"
)

type BubbleTFNominator struct {
	Command  TFCommand
	Amount   int64
	Actor    tongo.AccountID
	Contract tongo.AccountID
	Success  bool
}

func (b BubbleTFNominator) ToAction() *Action {
	return &Action{
		SmartContractExec: &SmartContractAction{
			TonAttached: b.Amount,
			Executor:    b.Actor,
			Contract:    b.Contract,
			Operation:   string(b.Command),
		},
		Type:    SmartContractExec,
		Success: b.Success,
	}

}

func FindTFNominatorAction(bubble *Bubble) bool {
	bubbleTx, ok := bubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	if bubbleTx.opCode == nil {
		return false
	}
	if !bubbleTx.account.Is(abi.TvPool) {
		return false
	}
	if bubbleTx.inputFrom == nil {
		return false
	}

	var command TFCommand
	amount := bubbleTx.inputAmount
	sender := bubbleTx.inputFrom.Address
	children := bubble.Children

	newBubble := Bubble{
		Accounts:  bubble.Accounts,
		ValueFlow: bubble.ValueFlow,
	}
	switch *bubbleTx.opCode {
	case abi.ElectorNewStakeMsgOpCode:
		command = TfDepositStakeRequest
	case abi.ElectorRecoverStakeRequestMsgOpCode:
		command = TfRecoverStakeRequest
	case 2:
		command = TfProcessPendingWithdrawRequests
		children = ProcessChildren(bubble.Children,
			func(child *Bubble) *Merge {
				tx, ok := child.Info.(BubbleTx)
				if !ok {
					return nil
				}
				if tx.opCode == nil && tx.account.Address == sender {
					// this is a send-excess transfer, let's eliminate it
					amount -= tx.inputAmount
					newBubble.ValueFlow.Merge(child.ValueFlow)
					return &Merge{children: child.Children}
				}
				return nil
			})
	case 6:
		command = TfUpdateValidatorSet
		children = ProcessChildren(bubble.Children,
			func(child *Bubble) *Merge {
				tx, ok := child.Info.(BubbleTx)
				if !ok {
					return nil
				}
				if tx.opCode == nil && tx.account.Address == sender {
					// this is a send-excess transfer, let's eliminate it
					amount -= tx.inputAmount
					newBubble.ValueFlow.Merge(child.ValueFlow)
					return &Merge{children: child.Children}
				}
				return nil
			})
	default:
		return false
	}
	newBubble.Info = BubbleTFNominator{
		Command:  command,
		Amount:   amount,
		Actor:    sender,
		Contract: bubbleTx.account.Address,
		Success:  bubbleTx.success,
	}
	newBubble.Children = children
	*bubble = newBubble
	return true

}
