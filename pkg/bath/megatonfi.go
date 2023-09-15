package bath

import (
	"github.com/tonkeeper/tongo/abi"
	"math/big"
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
		newAction.Dex = Megatonfi
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
