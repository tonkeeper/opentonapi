package bath

import (
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/abi"
)

// MegatonFiJettonSwap creates a BubbleJettonSwap if there is a jetton swap in a trace.
var MegatonFiJettonSwap = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{IsJettonTransfer, IsJettonReceiver(abi.MegatonfiRouter)},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		tx := bubble.Info.(BubbleJettonTransfer)
		newAction.UserWallet = tx.sender.Address
		newAction.In.Amount = big.Int(tx.amount)
		newAction.In.IsTon = tx.isWrappedTon
		newAction.Router = tx.recipient.Address
		newAction.In.JettonWallet = tx.senderWallet
		newAction.In.JettonMaster = tx.master
		newAction.Dex = references.Megatonfi
		return nil
	},
	Children: []Straw[BubbleJettonSwap]{
		{
			CheckFuncs: []bubbleCheck{IsJettonTransfer, IsJettonReceiver(abi.MegatonfiExchange)},
			Children: []Straw[BubbleJettonSwap]{
				{
					CheckFuncs: []bubbleCheck{IsJettonTransfer, IsJettonReceiver(abi.MegatonfiRouter)},
					Children: []Straw[BubbleJettonSwap]{
						{
							CheckFuncs: []bubbleCheck{IsJettonTransfer},
							Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
								tx := bubble.Info.(BubbleJettonTransfer)
								newAction.Success = tx.success
								newAction.Out.Amount = big.Int(tx.amount)
								newAction.Out.IsTon = tx.isWrappedTon
								newAction.Out.JettonWallet = tx.recipientWallet
								newAction.Out.JettonMaster = tx.master
								return nil
							},
						},
					},
				},
			},
		},
	},
}

var WtonMintStraw = Straw[BubbleJettonMint]{
	CheckFuncs: []bubbleCheck{IsTx, HasOpcode(0x77a33521)},
	Builder: func(newAction *BubbleJettonMint, bubble *Bubble) error {
		newAction.recipient = bubble.Info.(BubbleTx).account
		return nil
	},
	Children: []Straw[BubbleJettonMint]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonInternalTransferMsgOp)},
			Builder: func(newAction *BubbleJettonMint, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				body := tx.decodedBody.Value.(abi.JettonInternalTransferMsgBody)
				newAction.amount = body.Amount
				if tx.additionalInfo != nil {
					if master, ok := tx.additionalInfo.JettonMaster(tx.account.Address); ok {
						newAction.master = master
					}
				}
				newAction.recipientWallet = tx.account.Address
				newAction.success = tx.success
				return nil
			},
			Children: []Straw[BubbleJettonMint]{
				{
					CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonNotifyMsgOp)},
					Optional:   true,
				},
				{
					CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
					Optional:   true,
				},
			},
		},
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)},
			Optional:   true,
		},
	},
}
