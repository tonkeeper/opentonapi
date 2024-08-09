package bath

import (
	"fmt"
	"math/big"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

type BubbleJettonTransfer struct {
	sender, recipient             *Account
	senderWallet, recipientWallet tongo.AccountID
	master                        tongo.AccountID
	amount                        tlb.VarUInteger16
	success                       bool
	isWrappedTon                  bool
	payload                       abi.JettonPayload
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
			isWrappedTon:     b.isWrappedTon,
		},
		Success: b.success,
		Type:    JettonTransfer,
	}
	switch b.payload.SumType {
	case abi.TextCommentJettonOp:
		a.JettonTransfer.Comment = g.Pointer(string(b.payload.Value.(abi.TextCommentJettonPayload).Text))
	case abi.EncryptedTextCommentJettonOp:
		a.JettonTransfer.EncryptedComment = &EncryptedComment{
			CipherText:     b.payload.Value.(abi.EncryptedTextCommentJettonPayload).CipherText,
			EncryptionType: "simple",
		}
	case abi.EmptyJettonOp:
	default:
		if b.payload.SumType != abi.UnknownJettonOp {
			a.JettonTransfer.Comment = g.Pointer("Call: " + b.payload.SumType)
		} else if b.payload.OpCode != nil {
			a.JettonTransfer.Comment = g.Pointer(fmt.Sprintf("Call: 0x%08x", *b.payload.OpCode))
		}
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
		if tx.additionalInfo != nil {
			if master, ok := tx.additionalInfo.JettonMaster(tx.account.Address); ok {
				//todo: find why it doesn't set sometimes (maybe it already fixed but who knows?)
				newAction.master = master
			}
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

var JettonMintStrawGovernance = Straw[BubbleJettonMint]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonMintMsgOp), HasInterface(abi.JettonMaster)},
	Builder: func(newAction *BubbleJettonMint, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		msg := tx.decodedBody.Value.(abi.JettonMintMsgBody)
		dest, err := tongo.AccountIDFromTlb(msg.ToAddress)
		if err == nil && dest != nil {
			newAction.recipient = Account{Address: *dest}
		}
		return nil
	},
	SingleChild: &Straw[BubbleJettonMint]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonInternalTransferMsgOp), HasInterface(abi.JettonWallet)},
		Builder: func(newAction *BubbleJettonMint, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			msg := tx.decodedBody.Value.(abi.JettonInternalTransferMsgBody)
			newAction.amount = msg.Amount
			newAction.recipientWallet = tx.account.Address
			newAction.master = tx.inputFrom.Address
			newAction.success = tx.success
			return nil
		},
		SingleChild: &Straw[BubbleJettonMint]{
			Optional:   true,
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)},
		},
	},
}

var JettonTransferMinimalStraw = Straw[BubbleJettonTransfer]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.JettonWallet), HasOpcode(abi.JettonTransferMsgOpCode)},
	Builder: func(newAction *BubbleJettonTransfer, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.master, _ = tx.additionalInfo.JettonMaster(tx.account.Address)
		newAction.senderWallet = tx.account.Address
		newAction.sender = tx.inputFrom
		return nil
	},
	SingleChild: &Straw[BubbleJettonTransfer]{
		CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.JettonWallet), HasOperation(abi.JettonInternalTransferMsgOp)}, //todo: remove Or(). both should be in new indexer
		Optional:   true,
		Builder: func(newAction *BubbleJettonTransfer, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.recipientWallet = tx.account.Address
			if newAction.master.IsZero() {
				newAction.master, _ = tx.additionalInfo.JettonMaster(tx.account.Address)
			}
			newAction.success = tx.success
			body, _ := tx.decodedBody.Value.(abi.JettonInternalTransferMsgBody)
			newAction.amount = body.Amount
			return nil
		},
		ValueFlowUpdater: func(newAction *BubbleJettonTransfer, flow *ValueFlow) {
			if newAction.success {
				if newAction.recipient != nil {
					flow.AddJettons(newAction.recipient.Address, newAction.master, big.Int(newAction.amount))
				}
				if newAction.sender != nil {
					flow.SubJettons(newAction.sender.Address, newAction.master, big.Int(newAction.amount))
				}
			}
		},
		Children: []Straw[BubbleJettonTransfer]{
			{
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonNotifyMsgOp)},
				Builder: func(newAction *BubbleJettonTransfer, bubble *Bubble) error {
					tx := bubble.Info.(BubbleTx)
					newAction.success = true
					body := tx.decodedBody.Value.(abi.JettonNotifyMsgBody)
					newAction.amount = body.Amount
					newAction.payload = body.ForwardPayload.Value
					newAction.recipient = &tx.account
					if newAction.sender == nil {
						sender, err := ton.AccountIDFromTlb(body.Sender)
						if err == nil {
							newAction.sender = &Account{Address: *sender}
						}
					}
					return nil
				},
				ValueFlowUpdater: func(newAction *BubbleJettonTransfer, flow *ValueFlow) {
					if newAction.recipient != nil {
						flow.AddJettons(newAction.recipient.Address, newAction.master, big.Int(newAction.amount))
					}
					if newAction.sender != nil {
						flow.SubJettons(newAction.sender.Address, newAction.master, big.Int(newAction.amount))
					}
				},
				Optional: true,
			},
			{
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)},
				Optional:   true,
			},
		},
	},
}
