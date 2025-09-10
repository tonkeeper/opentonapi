package bath

import (
	"fmt"
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/abi"
)

var BidaskSwapStraw = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.BidaskSwapMsgOp), HasInterface(abi.BidaskPool), func(bubble *Bubble) bool {
		tx := bubble.Info.(BubbleTx)
		return tx.decodedBody.Value.(abi.BidaskSwapMsgBody).ToAddress == tx.inputFrom.Address.ToMsgAddress()
	}},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Dex = references.Bidask
		newAction.UserWallet = tx.inputFrom.Address
		newAction.Router = tx.account.Address
		body := tx.decodedBody.Value.(abi.BidaskSwapMsgBody)
		newAction.In.Amount = *big.NewInt(int64(body.NativeAmount))
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
		jettonTx := bubble.Info.(BubbleJettonTransfer)
		swap, ok := jettonTx.payload.Value.(abi.BidaskSwapJettonPayload)
		if !ok {
			return false
		}
		if swap.ToAddress != jettonTx.sender.Address.ToMsgAddress() {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		jettonTx := bubble.Info.(BubbleJettonTransfer)
		newAction.Dex = references.Bidask
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
					newAction.Out.Amount = *big.NewInt(int64(body.NativeAmount))
					newAction.Success = tx.success
					return nil
				},
			},
		},
	},
}

var BidaskJettonSwapStraw = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{IsJettonTransfer, func(bubble *Bubble) bool {
		jettonTx := bubble.Info.(BubbleJettonTransfer)
		swap, ok := jettonTx.payload.Value.(abi.BidaskSwapJettonPayload)
		if !ok {
			return false
		}
		if swap.ToAddress != jettonTx.sender.Address.ToMsgAddress() {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		jettonTx := bubble.Info.(BubbleJettonTransfer)
		newAction.Dex = references.Bidask
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
					newAction.Success = jettonTx.success
					return nil
				},
			},
		},
	},
}
