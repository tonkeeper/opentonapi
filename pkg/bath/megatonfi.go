package bath

import (
	"math/big"

	"github.com/tonkeeper/tongo/abi"
)

// MegatonFiJettonSwap creates a BubbleJettonSwap if there is a jetton swap in a trace.
var MegatonFiJettonSwap = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{IsJettonTransfer, IsJettonReceiver(abi.MegatonfiRouter)},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		tx := bubble.Info.(BubbleJettonTransfer)
		newAction.UserWallet = tx.sender.Address
		amountIn := big.Int(tx.amount)
		newAction.AmountIn = amountIn.Uint64()
		newAction.Router = tx.recipient.Address
		newAction.JettonWalletIn = tx.senderWallet
		newAction.JettonMasterIn = tx.master
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
								newAction.JettonWalletOut = tx.recipientWallet
								newAction.JettonMasterOut = tx.master
								amountOut := big.Int(tx.amount)
								newAction.AmountOut = amountOut.Uint64()
								return nil
							},
						},
					},
				},
			},
		},
	},
}
