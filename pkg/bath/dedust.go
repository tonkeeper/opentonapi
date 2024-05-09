package bath

import (
	"errors"
	"math/big"

	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/ton"
)

var DedustSwapStraw = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{IsJettonTransfer, JettonTransferOperation(abi.DedustSwapJettonOp), func(bubble *Bubble) bool {
		transfer := bubble.Info.(BubbleJettonTransfer)
		swap, ok := transfer.payload.Value.(abi.DedustSwapJettonPayload)
		if !ok {
			return false
		}
		to, err := ton.AccountIDFromTlb(swap.SwapParams.RecipientAddr)
		if err != nil {
			return false
		}
		if to == nil {
			return true
		}
		if transfer.sender == nil || transfer.sender.Address != *to {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		transfer := bubble.Info.(BubbleJettonTransfer)
		newAction.Dex = Dedust
		newAction.In.JettonMaster = transfer.master
		newAction.In.JettonWallet = transfer.senderWallet
		newAction.In.Amount = big.Int(transfer.amount)
		newAction.In.IsTon = transfer.isWrappedTon
		if transfer.payload.Value.(abi.DedustSwapJettonPayload).Step.Params.KindOut {
			return errors.New("dedust swap: wrong kind of limits") //not supported
		}
		newAction.Out.Amount = big.Int(transfer.payload.Value.(abi.DedustSwapJettonPayload).Step.Params.Limit)
		return nil
	},
	SingleChild: &Straw[BubbleJettonSwap]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.DedustSwapExternalMsgOp)},
		Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
			newAction.Router = bubble.Info.(BubbleTx).account.Address
			return nil
		},
		SingleChild: &Straw[BubbleJettonSwap]{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.DedustPayoutFromPoolMsgOp)},
			SingleChild: &Straw[BubbleJettonSwap]{
				CheckFuncs: []bubbleCheck{IsJettonTransfer},
				Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
					newAction.Success = true
					transfer := bubble.Info.(BubbleJettonTransfer)
					newAction.Out.JettonMaster = transfer.master
					newAction.Out.IsTon = transfer.isWrappedTon
					newAction.Out.Amount = big.Int(transfer.amount)
					newAction.Out.JettonWallet = transfer.recipientWallet
					return nil
				},
			},
		},
	},
}
