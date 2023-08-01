package bath

import "github.com/tonkeeper/tongo/abi"

// example: https://tonviewer.com/transaction/6d33487c44249d7844db8fac38a5cecf1502ec7e0c09d266e98e95a2b1be17b5
var DedustLPJettonMintStraw = Straw[BubbleJettonTransfer]{
	CheckFuncs: []bubbleCheck{IsTx, HasOpcode(0xb56b9598)}, //todo: switch to check interface to be jetton master and rename straw to be more generic
	Builder: func(newAction *BubbleJettonTransfer, bubble *Bubble) (err error) {
		tx := bubble.Info.(BubbleTx)
		newAction.sender = &tx.account
		newAction.master = tx.account.Address
		return nil
	},
	Children: []Straw[BubbleJettonTransfer]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonInternalTransferMsgOp)},
			Builder: func(newAction *BubbleJettonTransfer, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				msg := tx.decodedBody.Value.(abi.JettonInternalTransferMsgBody)
				newAction.amount = msg.Amount
				newAction.recipientWallet = tx.account.Address
				newAction.success = tx.success
				return nil
			},
			Children: []Straw[BubbleJettonTransfer]{
				{
					CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonNotifyMsgOp)},
					Builder: func(newAction *BubbleJettonTransfer, bubble *Bubble) error {
						tx := bubble.Info.(BubbleTx)
						newAction.recipient = &tx.account
						return nil
					},
				},
				{CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)}, Optional: true},
			},
		},
	},
}
