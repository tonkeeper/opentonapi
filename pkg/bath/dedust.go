package bath

import (
	"errors"
	"math/big"

	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/ton"
)

var dedustPrimitiveJettonInput = Straw[BubbleJettonSwap]{
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
		// A Dedust user may specify different address to receive resulting jettons. In that case it is not a swap.
		if transfer.sender == nil || transfer.sender.Address != *to {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		transfer := bubble.Info.(BubbleJettonTransfer)
		newAction.Success = true
		newAction.Dex = Dedust
		if transfer.sender != nil {
			newAction.UserWallet = transfer.sender.Address
		}
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
	},
}

var dedustPrimitiveTonInput = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.DedustSwapMsgOp), func(bubble *Bubble) bool {
		tx := bubble.Info.(BubbleTx)
		swap, ok := tx.decodedBody.Value.(abi.DedustSwapMsgBody)
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
		// A Dedust user may specify different address to receive resulting jettons. In that case it is not a swap.
		if tx.inputFrom == nil || tx.inputFrom.Address != *to {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		transfer := bubble.Info.(BubbleTx)
		newAction.Success = true
		newAction.Dex = Dedust
		if transfer.inputFrom != nil {
			newAction.UserWallet = transfer.inputFrom.Address
		}
		newAction.In.IsTon = true
		newAction.In.Amount.SetInt64(transfer.inputAmount)
		return nil
	},
	SingleChild: &Straw[BubbleJettonSwap]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.DedustSwapExternalMsgOp), HasInterface(abi.DedustPool)},
		Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
			newAction.Router = bubble.Info.(BubbleTx).account.Address
			return nil
		},
	},
}

var dedustPrimitiveJettonOutput = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.DedustPayoutFromPoolMsgOp), HasInterface(abi.DedustVault)},
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
}

var dedustPrimitiveTonOutput = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.DedustPayoutFromPoolMsgOp), HasInterface(abi.DedustVault)},
	SingleChild: &Straw[BubbleJettonSwap]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.DedustPayoutMsgOp)},
		Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
			transfer := bubble.Info.(BubbleTx)
			newAction.Out.IsTon = true
			newAction.Out.Amount.SetInt64(transfer.inputAmount)
			return nil
		},
	},
}

var DedustSwapJettonsStraw Straw[BubbleJettonSwap]
var DedustSwapToTONStraw Straw[BubbleJettonSwap]
var DedustSwapFromTONStraw Straw[BubbleJettonSwap]

var DedustTwoHopesSwapJettonsStraw Straw[BubbleJettonSwap]
var DedustTwoHopesSwapToTONStraw Straw[BubbleJettonSwap]
var DedustTwoHopesSwapFromTONStraw Straw[BubbleJettonSwap]

func init() {
	DedustSwapJettonsStraw = *copyStraw(dedustPrimitiveJettonInput)
	DedustSwapJettonsStraw.SingleChild.SingleChild = copyStraw(dedustPrimitiveJettonOutput)

	DedustSwapToTONStraw = *copyStraw(dedustPrimitiveJettonInput)
	DedustSwapToTONStraw.SingleChild.SingleChild = copyStraw(dedustPrimitiveTonOutput)

	DedustSwapFromTONStraw = *copyStraw(dedustPrimitiveTonInput)
	DedustSwapFromTONStraw.SingleChild.SingleChild = &Straw[BubbleJettonSwap]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.DedustPayoutFromPoolMsgOp), HasInterface(abi.DedustVault)},
		SingleChild: &Straw[BubbleJettonSwap]{
			CheckFuncs: []bubbleCheck{Or(IsJettonTransfer, IsTx)},
			Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
				if IsJettonTransfer(bubble) {
					transfer := bubble.Info.(BubbleJettonTransfer)
					newAction.Out.JettonMaster = transfer.master
					newAction.Out.Amount = big.Int(transfer.amount)
					newAction.Out.JettonWallet = transfer.recipientWallet
					newAction.UserWallet = transfer.recipient.Address
				} else {
					transfer := bubble.Info.(BubbleTx)
					newAction.Success = false
					newAction.Out.IsTon = true
					newAction.Out.Amount.SetInt64(transfer.inputAmount)
					newAction.Out.JettonWallet = transfer.inputFrom.Address
					newAction.UserWallet = transfer.account.Address
				}
				return nil
			},
		},
	}

	DedustTwoHopesSwapJettonsStraw = *copyStraw(dedustPrimitiveJettonInput)
	DedustTwoHopesSwapJettonsStraw.SingleChild.SingleChild = &Straw[BubbleJettonSwap]{
		CheckFuncs:  []bubbleCheck{IsTx, HasOperation(abi.DedustSwapPeerMsgOp), HasInterface(abi.DedustPool)},
		SingleChild: copyStraw(dedustPrimitiveJettonOutput),
	}

	DedustTwoHopesSwapToTONStraw = *copyStraw(dedustPrimitiveJettonInput)
	DedustTwoHopesSwapToTONStraw.SingleChild.SingleChild = &Straw[BubbleJettonSwap]{
		CheckFuncs:  []bubbleCheck{IsTx, HasOperation(abi.DedustSwapPeerMsgOp), HasInterface(abi.DedustPool)},
		SingleChild: copyStraw(dedustPrimitiveTonOutput),
	}

	DedustTwoHopesSwapFromTONStraw = *copyStraw(dedustPrimitiveTonInput)
	DedustTwoHopesSwapFromTONStraw.SingleChild.SingleChild = &Straw[BubbleJettonSwap]{
		CheckFuncs:  []bubbleCheck{IsTx, HasInterface(abi.DedustPool)},
		SingleChild: copyStraw(dedustPrimitiveJettonOutput),
	}

	DefaultStraws = append(DefaultStraws,
		DedustSwapJettonsStraw,
		DedustSwapToTONStraw,
		DedustSwapFromTONStraw,
		DedustTwoHopesSwapJettonsStraw,
		DedustTwoHopesSwapToTONStraw,
		DedustTwoHopesSwapFromTONStraw)

}
