package bath

import (
	"fmt"
	"math/big"

	"github.com/tonkeeper/tongo/abi"
)

var BidaskSwapStraw = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.BidaskSwapMsgOp), HasInterface(abi.BidaskPool), func(bubble *Bubble) bool {
		tx, ok := bubble.Info.(BubbleTx)
		if !ok {
			return false
		}
		swap, ok := tx.decodedBody.Value.(abi.BidaskSwapMsgBody)
		if !ok {
			return false
		}
		if swap.ForwardPayload != nil {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Dex = Bidask
		newAction.UserWallet = tx.inputFrom.Address
		newAction.Router = tx.account.Address
		body, ok := tx.decodedBody.Value.(abi.BidaskSwapMsgBody)
		if !ok {
			return fmt.Errorf("body is not a mooncx swap body")
		}
		amount := new(big.Int)
		amount.SetUint64(uint64(body.NativeAmount))
		newAction.In.Amount = *amount
		newAction.In.IsTon = true
		return nil
	},
	SingleChild: &Straw[BubbleJettonSwap]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.BidaskInternalSwapMsgOp), HasInterface(abi.BidaskRange)},
		SingleChild: &Straw[BubbleJettonSwap]{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.BidaskSwapSuccessMsgOp), HasInterface(abi.BidaskPool)},
			SingleChild: &Straw[BubbleJettonSwap]{
				CheckFuncs: []bubbleCheck{IsJettonTransfer},
				Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
					jettonTx := bubble.Info.(BubbleJettonTransfer)
					newAction.Out.JettonMaster = jettonTx.master
					newAction.Out.JettonWallet = jettonTx.recipientWallet
					newAction.Out.Amount = big.Int(jettonTx.amount)
					newAction.Success = jettonTx.success
					return nil
				},
			},
		},
	},
}

var BidaskSwapStrawReverse = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{IsJettonTransfer, func(bubble *Bubble) bool {
		jettonTx, ok := bubble.Info.(BubbleJettonTransfer)
		if !ok {
			return false
		}
		swap, ok := jettonTx.payload.Value.(abi.BidaskSwapJettonPayload)
		if !ok {
			return false
		}
		if swap.ForwardPayload != nil {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		jettonTx := bubble.Info.(BubbleJettonTransfer)
		newAction.Dex = Bidask
		newAction.UserWallet = jettonTx.sender.Address
		newAction.Router = jettonTx.recipient.Address
		newAction.In.JettonMaster = jettonTx.master
		newAction.In.JettonWallet = jettonTx.senderWallet
		newAction.In.Amount = big.Int(jettonTx.amount)
		return nil
	},
	SingleChild: &Straw[BubbleJettonSwap]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.BidaskInternalSwapMsgOp), HasInterface(abi.BidaskRange)},
		SingleChild: &Straw[BubbleJettonSwap]{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.BidaskSwapSuccessMsgOp), HasInterface(abi.BidaskPool)},
			SingleChild: &Straw[BubbleJettonSwap]{
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.BidaskNativeTransferNotificationMsgOp)},
				Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
					tx := bubble.Info.(BubbleTx)
					newAction.Out.IsTon = true
					body, ok := tx.decodedBody.Value.(abi.BidaskNativeTransferNotificationMsgBody)
					if !ok {
						return fmt.Errorf("body is not a bidask native transfer notification")
					}
					amount := new(big.Int)
					amount.SetUint64(uint64(body.NativeAmount))
					newAction.Out.Amount = *amount
					newAction.Success = tx.success
					return nil
				},
			},
		},
	},
}

var BidaskJettonSwapStraw = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{IsJettonTransfer, func(bubble *Bubble) bool {
		jettonTx, ok := bubble.Info.(BubbleJettonTransfer)
		if !ok {
			return false
		}
		swap, ok := jettonTx.payload.Value.(abi.BidaskSwapJettonPayload)
		if !ok {
			return false
		}
		if swap.ForwardPayload != nil {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		jettonTx := bubble.Info.(BubbleJettonTransfer)
		newAction.Dex = Bidask
		newAction.UserWallet = jettonTx.sender.Address
		newAction.Router = jettonTx.recipient.Address
		newAction.In.JettonMaster = jettonTx.master
		newAction.In.JettonWallet = jettonTx.senderWallet
		newAction.In.Amount = big.Int(jettonTx.amount)
		return nil
	},
	SingleChild: &Straw[BubbleJettonSwap]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.BidaskInternalSwapMsgOp), HasInterface(abi.BidaskRange)},
		SingleChild: &Straw[BubbleJettonSwap]{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.BidaskSwapSuccessMsgOp), HasInterface(abi.BidaskPool)},
			SingleChild: &Straw[BubbleJettonSwap]{
				CheckFuncs: []bubbleCheck{IsJettonTransfer},
				Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
					jettonTx := bubble.Info.(BubbleJettonTransfer)
					newAction.Out.JettonMaster = jettonTx.master
					newAction.Out.JettonWallet = jettonTx.recipientWallet
					newAction.Out.Amount = big.Int(jettonTx.amount)
					return nil
				},
			},
		},
	},
}
