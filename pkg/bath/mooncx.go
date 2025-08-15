package bath

import (
	"fmt"
	"math/big"

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
		if swap.SwapParams.NextFulfill != nil {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Dex = Mooncx
		newAction.UserWallet = tx.inputFrom.Address
		newAction.Router = tx.account.Address
		body, ok := tx.decodedBody.Value.(abi.MoonSwapMsgBody)
		if !ok {
			return fmt.Errorf("body is not a mooncx swap body")
		}
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
			newAction.Out.JettonWallet = tx.senderWallet
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
		if swap.SwapParams.NextFulfill != nil {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		tx := bubble.Info.(BubbleJettonTransfer)
		newAction.Dex = Mooncx
		newAction.UserWallet = tx.senderWallet
		newAction.Router = tx.recipient.Address
		newAction.In.JettonMaster = tx.master
		newAction.In.JettonWallet = tx.senderWallet
		amount := big.Int(tx.amount)
		newAction.In.Amount = amount
		body := tx.payload.Value.(abi.MoonSwapJettonPayload)
		newAction.Out.Amount = big.Int(body.SwapParams.MinOut)
		return nil
	},
	SingleChild: &Straw[BubbleJettonSwap]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.MoonSwapSucceedJettonOp), HasInterface(abi.Wallet)},
		Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.UserWallet = tx.account.Address
			newAction.Out.IsTon = true
			newAction.Success = tx.success
			return nil
		},
	},
}
