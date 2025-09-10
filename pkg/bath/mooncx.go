package bath

import (
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/abi"
)

var MooncxSwapStraw = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.MoonSwapMsgOp), HasInterface(abi.MoonPool), func(bubble *Bubble) bool {
		tx, ok := bubble.Info.(BubbleTx)
		if !ok {
			return false
		}
		swap, ok := tx.decodedBody.Value.(abi.MoonSwapMsgBody)
		if !ok {
			return false
		}
		if swap.SwapParams.NextFulfill != nil && swap.SwapParams.NextFulfill.Recipient != tx.inputFrom.Address.ToMsgAddress() {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Dex = references.Mooncx
		newAction.UserWallet = tx.inputFrom.Address
		newAction.Router = tx.account.Address
		body := tx.decodedBody.Value.(abi.MoonSwapMsgBody)
		amount := big.Int(body.Amount)
		newAction.In.Amount = amount
		newAction.In.IsTon = true
		return nil
	},
	SingleChild: &Straw[BubbleJettonSwap]{
		CheckFuncs: []bubbleCheck{IsJettonTransfer},
		Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
			tx := bubble.Info.(BubbleJettonTransfer)
			newAction.Out.JettonMaster = tx.master
			newAction.Out.JettonWallet = tx.recipientWallet
			amount := big.Int(tx.amount)
			newAction.Out.Amount = amount
			newAction.Success = tx.success
			return nil
		},
	},
}

var MooncxSwapStrawReverse = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{func(bubble *Bubble) bool {
		tx, ok := bubble.Info.(BubbleJettonTransfer)
		if !ok {
			return false
		}
		swap, ok := tx.payload.Value.(abi.MoonSwapJettonPayload)
		if !ok {
			return false
		}
		if swap.SwapParams.NextFulfill != nil && swap.SwapParams.NextFulfill.Recipient != tx.sender.Address.ToMsgAddress() {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		tx := bubble.Info.(BubbleJettonTransfer)
		newAction.Dex = references.Mooncx
		newAction.UserWallet = tx.sender.Address
		newAction.Router = tx.recipient.Address
		newAction.In.JettonMaster = tx.master
		newAction.In.JettonWallet = tx.senderWallet
		amount := big.Int(tx.amount)
		newAction.In.Amount = amount
		body := tx.payload.Value.(abi.MoonSwapJettonPayload)
		newAction.Out.Amount = big.Int(body.SwapParams.MinOut)
		newAction.Out.IsTon = true
		return nil
	},
	SingleChild: &Straw[BubbleJettonSwap]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.MoonSwapSucceedMsgOp)},
		Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.Success = tx.success
			return nil
		},
	},
}
