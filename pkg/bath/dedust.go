package bath

import (
	"errors"
	"math/big"
	"slices"

	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/ton"
)

func (s UniversalDedustStraw) Merge(b *Bubble) bool {
	var steps abi.DedustSwapStep
	var sender ton.AccountID
	var out, in assetTransfer
	if IsTx(b) && b.Info.(BubbleTx).operation(abi.DedustSwapMsgOp) {
		tx := b.Info.(BubbleTx)
		if tx.inputFrom == nil {
			return false
		}
		swap := tx.decodedBody.Value.(abi.DedustSwapMsgBody)
		steps = swap.Step
		sender = tx.inputFrom.Address
		in.IsTon = true
		in.Amount.SetInt64(int64(swap.Amount))
	} else if IsJettonTransfer(b) && JettonTransferOperation(abi.DedustSwapJettonOp)(b) {
		transfer := b.Info.(BubbleJettonTransfer)
		if transfer.sender == nil {
			return false
		}
		swap := transfer.payload.Value.(abi.DedustSwapJettonPayload)
		steps = swap.Step
		sender = transfer.sender.Address
		in.Amount = big.Int(transfer.amount)
		in.JettonMaster = transfer.master
		in.JettonWallet = transfer.senderWallet
	} else {
		return false
	}
	expectStepsCount := s.countSteps(steps)
	realStepsCount, failedSteps, swapsBubbles, payoutCommandBubble, err := s.recursiveProcessSteps(b)
	if err != nil || failedSteps != 0 || expectStepsCount != realStepsCount {
		return false
	}
	if !IsTx(payoutCommandBubble) || len(payoutCommandBubble.Children) < 1 {
		return false
	}
	payoutCommand, ok := payoutCommandBubble.Info.(BubbleTx).decodedBody.Value.(abi.DedustPayoutFromPoolMsgBody)
	if !ok {
		return false
	}
	out.Amount = big.Int(payoutCommand.Amount)
	payoutBubble := payoutCommandBubble.Children[0]
	var payoutDestination Account
	if IsTx(payoutBubble) && payoutBubble.Info.(BubbleTx).operation(abi.DedustPayoutMsgOp) {
		out.IsTon = true
		payoutDestination = payoutBubble.Info.(BubbleTx).account
	} else if IsJettonTransfer(payoutBubble) {
		transfer := payoutBubble.Info.(BubbleJettonTransfer)
		out.JettonMaster = transfer.master
		out.JettonWallet = transfer.senderWallet
		out.Amount = big.Int(transfer.amount)
		if transfer.recipient == nil {
			return false
		}
		payoutDestination = *transfer.recipient
	} else {
		return false
	}

	//omniston referral account
	// кажется потом можно будет вынести в отдельную функцию, но пока не хочется
	if payoutDestination.Is(abi.OmnistonReferral) && len(payoutBubble.Children) == 2 {
		if out.IsTon {
			referalBubble := payoutBubble.Children[0]
			payoutBubble2 := payoutBubble.Children[1]

			referalTx, ok1 := referalBubble.Info.(BubbleTx)
			payoutBubble2tx, ok2 := payoutBubble2.Info.(BubbleTx)
			if !(ok1 && ok2) || !(referalTx.operation(abi.ExcessMsgOp) && payoutBubble2tx.operation(abi.ExcessMsgOp)) {
				return false
			}
			if referalTx.inputAmount < payoutBubble2tx.inputAmount {
				swapsBubbles = append(swapsBubbles, payoutBubble2)
				out.Amount.SetInt64(payoutBubble2tx.inputAmount)
				payoutDestination = payoutBubble2tx.account
			} else { //видимо перепутались рефералка и выплата. рефералка меньше чем выплата
				swapsBubbles = append(swapsBubbles, referalBubble)
				out.Amount.SetInt64(referalTx.inputAmount)
				payoutDestination = referalTx.account
			}
		} else {
			referalBubble := payoutBubble.Children[0]
			payoutBubble2 := payoutBubble.Children[1]

			referalTransfer, ok1 := referalBubble.Info.(BubbleTx)
			if !ok1 {
				referalBubble, payoutBubble2 = payoutBubble2, payoutBubble
				referalTransfer, ok1 = referalBubble.Info.(BubbleTx)
				if !ok1 || (referalTransfer.opCode == nil || *referalTransfer.opCode != 0x603f6e78) {
					return false
				}
			}
			payoutBubble2Transfer, ok2 := payoutBubble2.Info.(BubbleJettonTransfer)
			if !ok2 || payoutBubble2Transfer.recipient == nil {
				return false
			}
			payoutDestination = *payoutBubble2Transfer.recipient
			out.Amount = big.Int(payoutBubble2Transfer.amount)
			swapsBubbles = append(swapsBubbles, payoutBubble2)
			if len(referalBubble.Children) == 1 && IsTx(referalBubble.Children[0]) && referalBubble.Children[0].Info.(BubbleTx).operation(abi.ExcessMsgOp) {
				swapsBubbles = append(swapsBubbles, referalBubble.Children[0])
				swapsBubbles = append(swapsBubbles, referalBubble)
			}
		}
	}

	// проверяем что не подменен адрес получателя свапа
	if payoutDestination.Address != sender {
		return false
	}

	//закончили все проверки и собрали данные. билдим выходной пузырь и мержим
	toMerge := append(swapsBubbles, payoutCommandBubble, payoutBubble)
	var newChildren []*Bubble
	for i := range b.Children {
		if !slices.Contains(toMerge, b.Children[i]) {
			newChildren = append(newChildren, b.Children[i])
		}
	}
	for i := range toMerge {
		b.ValueFlow.Merge(toMerge[i].ValueFlow)
		b.Accounts = append(b.Accounts, toMerge[i].Accounts...)
		b.Transaction = append(b.Transaction, toMerge[i].Transaction...)
		for j := range toMerge[i].Children { //прикрепляем детей от удаляемых баблов напрямую к родителю
			tb := toMerge[i].Children[j]
			if !slices.Contains(toMerge, tb) {
				newChildren = append(newChildren, tb)
			}
		}
	}
	b.Children = newChildren
	b.Info = BubbleJettonSwap{
		Dex:        references.Dedust,
		UserWallet: sender,
		Router:     swapsBubbles[0].Info.(BubbleTx).account.Address,
		Out:        out,
		In:         in,
		Success:    true,
	}
	return true
}

// recursiveProcessSteps returns number of steps, number of fails, list of all txBubbles on pools, and last Bubble with payout command
func (s UniversalDedustStraw) recursiveProcessSteps(b *Bubble) (int, int, []*Bubble, *Bubble, error) {
	if len(b.Children) < 1 {
		return 0, 0, nil, nil, errors.New("unexpected end of swap")
	}
	child := b.Children[0]
	if !IsTx(child) {
		return 0, 0, nil, nil, errors.New("unexpected end of swap")
	}
	tx := child.Info.(BubbleTx)
	if tx.account.Is(abi.DedustPool) {
		step, fails, deepSwaps, endBubble, err := s.recursiveProcessSteps(child)
		if tx.findExternalOut(abi.DedustSwapMsgOp) == nil {
			fails++
		}
		return step + 1, fails, append(deepSwaps, child), endBubble, err
	}
	return 0, 0, nil, child, nil
}

func (s UniversalDedustStraw) countSteps(step abi.DedustSwapStep) int {
	if step.Params.Next == nil {
		return 1
	}
	return s.countSteps(*step.Params.Next) + 1
}

type UniversalDedustStraw struct{}
