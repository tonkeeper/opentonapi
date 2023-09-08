package bath

import (
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
)

type BubbleJettonTransfer struct {
	sender, recipient             *Account
	senderWallet, recipientWallet tongo.AccountID
	master                        tongo.AccountID
	amount                        tlb.VarUInteger16
	success                       bool
	payload                       any
}

func (b BubbleJettonTransfer) ToAction() (action *Action) {
	a := Action{
		JettonTransfer: &JettonTransferAction{
			Jetton:           b.master,
			Recipient:        b.recipient.Addr(),
			Sender:           b.sender.Addr(),
			RecipientsWallet: b.recipientWallet,
			SendersWallet:    b.senderWallet,
			Amount:           b.amount,
		},
		Success: b.success,
		Type:    JettonTransfer,
	}
	switch c := b.payload.(type) {
	case string:
		a.JettonTransfer.Comment = &c
	case EncryptedComment:
		a.JettonTransfer.EncryptedComment = &c
	}
	return &a
}

type BubbleJettonMint struct {
	recipient       Account
	recipientWallet tongo.AccountID
	master          tongo.AccountID
	amount          tlb.VarUInteger16
	success         bool
}

func (b BubbleJettonMint) ToAction() (action *Action) {
	a := Action{
		JettonMint: &JettonMintAction{
			Jetton:           b.master,
			Recipient:        b.recipient.Address,
			RecipientsWallet: b.recipientWallet,
			Amount:           b.amount,
		},
		Success: b.success,
		Type:    JettonMint,
	}
	return &a
}

type BubbleJettonBurn struct {
	sender       Account
	senderWallet tongo.AccountID
	master       tongo.AccountID
	amount       tlb.VarUInteger16
	success      bool
}

func (b BubbleJettonBurn) ToAction() (action *Action) {
	a := Action{
		JettonBurn: &JettonBurnAction{
			Jetton:        b.master,
			Sender:        b.sender.Address,
			SendersWallet: b.senderWallet,
			Amount:        b.amount,
		},
		Success: b.success,
		Type:    JettonBurn,
	}
	return &a
}

// DedustLPJettonMintStraw example: https://tonviewer.com/transaction/6d33487c44249d7844db8fac38a5cecf1502ec7e0c09d266e98e95a2b1be17b5
var DedustLPJettonMintStraw = Straw[BubbleJettonMint]{
	CheckFuncs: []bubbleCheck{IsTx, Or(HasOpcode(0xb56b9598), HasOpcode(0x1674b0a0))}, //todo: switch to check interface to be jetton master and rename straw to be more generic
	Builder: func(newAction *BubbleJettonMint, bubble *Bubble) (err error) {
		tx := bubble.Info.(BubbleTx)
		newAction.master = tx.account.Address
		return nil
	},
	Children: []Straw[BubbleJettonMint]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonInternalTransferMsgOp)},
			Builder: func(newAction *BubbleJettonMint, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				msg := tx.decodedBody.Value.(abi.JettonInternalTransferMsgBody)
				newAction.amount = msg.Amount
				newAction.recipientWallet = tx.account.Address
				newAction.success = tx.success
				return nil
			},
			Children: []Straw[BubbleJettonMint]{
				{
					CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonNotifyMsgOp)},
					Builder: func(newAction *BubbleJettonMint, bubble *Bubble) error {
						tx := bubble.Info.(BubbleTx)
						newAction.recipient = tx.account
						return nil
					},
				},
				{CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)}, Optional: true},
			},
		},
	},
}

var JettonBurnStraw = Straw[BubbleJettonBurn]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonBurnMsgOp)},
	Builder: func(newAction *BubbleJettonBurn, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		msg := tx.decodedBody.Value.(abi.JettonBurnMsgBody)
		newAction.amount = msg.Amount
		if tx.inputFrom != nil {
			newAction.sender = *tx.inputFrom
		}
		if tx.additionalInfo.JettonMaster != nil { //todo: find why it doesn't set https://dev.tonviewer.com/transaction/b563b85a8e56bad6333a5999a34137f302b14764b4ad1ebb4ecbecab2e16fa32
			newAction.master = *tx.additionalInfo.JettonMaster
		}
		newAction.senderWallet = tx.account.Address
		newAction.success = tx.success
		return nil
	},
	SingleChild: &Straw[BubbleJettonBurn]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonBurnNotificationMsgOp)},
		Builder: func(newAction *BubbleJettonBurn, bubble *Bubble) error { //todo: remove after fixing additionalInfo few lines above
			newAction.master = bubble.Info.(BubbleTx).account.Address
			return nil
		},
		Optional: true,
	},
}
