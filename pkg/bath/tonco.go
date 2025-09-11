package bath

import (
	"fmt"
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/abi"
)

var ToncoSwapStraw = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{IsJettonTransfer},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		tx := bubble.Info.(BubbleJettonTransfer)
		newAction.Dex = references.Tonco
		newAction.UserWallet = tx.sender.Address
		newAction.Router = tx.recipient.Address
		newAction.In.IsTon = tx.isWrappedTon
		newAction.In.Amount = big.Int(tx.amount)
		if !newAction.In.IsTon {
			newAction.In.JettonWallet = tx.senderWallet
			newAction.In.JettonMaster = tx.master
		}
		return nil
	},
	SingleChild: &Straw[BubbleJettonSwap]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.Poolv3SwapMsgOp), HasInterface(abi.ToncoPool)},
		Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			body, ok := tx.decodedBody.Value.(abi.Poolv3SwapMsgBody)
			if !ok {
				return fmt.Errorf("not a tonco pool_v3_swap_msg_body")
			}
			if body.PayloadsCell.TargetAddress != newAction.UserWallet.ToMsgAddress() {
				return fmt.Errorf("not a user wallet")
			}
			return nil
		},
		SingleChild: &Straw[BubbleJettonSwap]{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.PayToMsgOp), HasInterface(abi.ToncoRouter), func(bubble *Bubble) bool {
				tx := bubble.Info.(BubbleTx)
				body, ok := tx.decodedBody.Value.(abi.PayToMsgBody)
				if !ok {
					return false
				}
				if body.PayTo.SumType != "PayToCode200" && body.PayTo.SumType != "PayToCode230" { // 200 - swap, 201 - burn
					return false
				}
				return true
			}},
			Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				// exit code 200 means successful swap, 230 - failed swap
				newAction.Success = tx.decodedBody.Value.(abi.PayToMsgBody).PayTo.SumType == "PayToCode200"
				return nil
			},
			SingleChild: &Straw[BubbleJettonSwap]{
				CheckFuncs: []bubbleCheck{IsJettonTransfer},
				Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
					tx := bubble.Info.(BubbleJettonTransfer)
					newAction.Out.IsTon = tx.isWrappedTon
					newAction.Out.Amount = big.Int(tx.amount)
					if !newAction.Out.IsTon {
						newAction.Out.JettonWallet = tx.recipientWallet
						newAction.Out.JettonMaster = tx.master
					}
					return nil
				},
			},
		},
	},
}
