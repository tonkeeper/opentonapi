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
	// convert jetton transfer to ton transfer if token is pTON
	if b.isWrappedTon && b.recipientWallet.IsZero() {
		amount := big.Int(b.amount)
		a := Action{
			TonTransfer: &TonTransferAction{
				Amount:    amount.Int64(),
				Recipient: b.recipient.Address,
				Sender:    b.sender.Address,
			},
			Success: b.success,
			Type:    TonTransfer,
		}

		return &a
	}

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
	a.JettonTransfer.PayloadFromABI(b.payload)
	return &a
}

func (jta *JettonTransferAction) PayloadFromABI(payload abi.JettonPayload) {
	switch payload.SumType {
	case abi.TextCommentJettonOp:
		jta.Comment = g.Pointer(string(payload.Value.(abi.TextCommentJettonPayload).Text))
	case abi.EncryptedTextCommentJettonOp:
		jta.EncryptedComment = &EncryptedComment{
			CipherText:     payload.Value.(abi.EncryptedTextCommentJettonPayload).CipherText,
			EncryptionType: "simple",
		}
	case abi.EmptyJettonOp:
	default:
		if payload.SumType != abi.UnknownJettonOp {
			jta.Comment = g.Pointer("Call: " + payload.SumType)
		} else if payload.OpCode != nil {
			jta.Comment = g.Pointer(fmt.Sprintf("Call: 0x%08x", *payload.OpCode))
		}
	}
}

type BubbleFlawedJettonTransfer struct {
	sender, recipient             *Account
	senderWallet, recipientWallet tongo.AccountID
	master                        tongo.AccountID
	sentAmount                    tlb.VarUInteger16
	receivedAmount                tlb.VarUInteger16
	success                       bool
	payload                       abi.JettonPayload
}

func (b BubbleFlawedJettonTransfer) ToAction() (action *Action) {
	a := Action{
		FlawedJettonTransfer: &FlawedJettonTransferAction{
			Jetton:           b.master,
			Recipient:        b.recipient.Addr(),
			Sender:           b.sender.Addr(),
			RecipientsWallet: b.recipientWallet,
			SendersWallet:    b.senderWallet,
			SentAmount:       b.sentAmount,
			ReceivedAmount:   b.receivedAmount,
		},
		Success: b.success,
		Type:    FlawedJettonTransfer,
	}
	a.FlawedJettonTransfer.PayloadFromABI(b.payload)
	return &a
}

func (fjta *FlawedJettonTransferAction) PayloadFromABI(payload abi.JettonPayload) {
	switch payload.SumType {
	case abi.TextCommentJettonOp:
		fjta.Comment = g.Pointer(string(payload.Value.(abi.TextCommentJettonPayload).Text))
	case abi.EncryptedTextCommentJettonOp:
		fjta.EncryptedComment = &EncryptedComment{
			CipherText:     payload.Value.(abi.EncryptedTextCommentJettonPayload).CipherText,
			EncryptionType: "simple",
		}
	case abi.EmptyJettonOp:
	default:
		if payload.SumType != abi.UnknownJettonOp {
			fjta.Comment = g.Pointer("Call: " + payload.SumType)
		} else if payload.OpCode != nil {
			fjta.Comment = g.Pointer(fmt.Sprintf("Call: 0x%08x", *payload.OpCode))
		}
	}
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

// JettonMintFromMasterStraw example: https://tonviewer.com/transaction/6d33487c44249d7844db8fac38a5cecf1502ec7e0c09d266e98e95a2b1be17b5
var JettonMintFromMasterStraw = Straw[BubbleJettonMint]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonInternalTransferMsgOp), HasInterface(abi.JettonWallet), func(bubble *Bubble) bool {
		tx := bubble.Info.(BubbleTx)
		return tx.inputFrom != nil && tx.inputFrom.Is(abi.JettonMaster)
	}},

	Builder: func(newAction *BubbleJettonMint, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		msg := tx.decodedBody.Value.(abi.JettonInternalTransferMsgBody)
		newAction.amount = msg.Amount
		newAction.master = tx.inputFrom.Address
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
			ValueFlowUpdater: func(newAction *BubbleJettonMint, flow *ValueFlow) {
				if newAction.success {
					flow.AddJettons(newAction.recipient.Address, newAction.master, big.Int(newAction.amount))
				}
			},
		},
		{CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)}, Optional: true},
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
		ValueFlowUpdater: func(newAction *BubbleJettonBurn, flow *ValueFlow) {
			if newAction.success {
				flow.SubJettons(newAction.sender.Address, newAction.master, big.Int(newAction.amount))
			}
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

var FlawedJettonTransferMinimalStraw = Straw[BubbleFlawedJettonTransfer]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.JettonWallet), HasOpcode(abi.JettonTransferMsgOpCode), func(bubble *Bubble) bool {
		// Check that sent amount is not the same as received one
		if len(bubble.Children) != 1 {
			return false
		}

		currTx := bubble.Info.(BubbleTx)
		transferBody, _ := currTx.decodedBody.Value.(abi.JettonTransferMsgBody)
		transferAmount := big.Int(transferBody.Amount)

		internalTransferTx := bubble.Children[0].Info.(BubbleTx)
		internalTransfer, ok := internalTransferTx.decodedBody.Value.(abi.JettonInternalTransferMsgBody)
		if !ok {
			return false
		}
		internalTransferAmount := big.Int(internalTransfer.Amount)

		if transferAmount.Cmp(&internalTransferAmount) == 0 {
			return false
		}

		return true
	}},
	Builder: func(newAction *BubbleFlawedJettonTransfer, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.master, _ = tx.additionalInfo.JettonMaster(tx.account.Address)
		newAction.senderWallet = tx.account.Address
		newAction.sender = tx.inputFrom
		body, _ := tx.decodedBody.Value.(abi.JettonTransferMsgBody)
		newAction.sentAmount = body.Amount
		return nil
	},
	SingleChild: &Straw[BubbleFlawedJettonTransfer]{
		CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.JettonWallet), HasOperation(abi.JettonInternalTransferMsgOp)},
		Optional:   true,
		Builder: func(newAction *BubbleFlawedJettonTransfer, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.recipientWallet = tx.account.Address
			if newAction.master.IsZero() {
				newAction.master, _ = tx.additionalInfo.JettonMaster(tx.account.Address)
			}
			newAction.success = tx.success
			body, _ := tx.decodedBody.Value.(abi.JettonInternalTransferMsgBody)
			newAction.receivedAmount = body.Amount
			return nil
		},
		ValueFlowUpdater: func(newAction *BubbleFlawedJettonTransfer, flow *ValueFlow) {
			if newAction.success {
				if newAction.recipient != nil {
					flow.AddJettons(newAction.recipient.Address, newAction.master, big.Int(newAction.receivedAmount))
				}
				if newAction.sender != nil {
					flow.SubJettons(newAction.sender.Address, newAction.master, big.Int(newAction.sentAmount))
				}
			}
		},
		Children: []Straw[BubbleFlawedJettonTransfer]{
			{
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonNotifyMsgOp)},
				Builder: func(newAction *BubbleFlawedJettonTransfer, bubble *Bubble) error {
					tx := bubble.Info.(BubbleTx)
					newAction.success = true
					body := tx.decodedBody.Value.(abi.JettonNotifyMsgBody)
					newAction.receivedAmount = body.Amount
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
				ValueFlowUpdater: func(newAction *BubbleFlawedJettonTransfer, flow *ValueFlow) {
					if newAction.recipient != nil {
						flow.AddJettons(newAction.recipient.Address, newAction.master, big.Int(newAction.receivedAmount))
					}
					if newAction.sender != nil {
						flow.SubJettons(newAction.sender.Address, newAction.master, big.Int(newAction.sentAmount))
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
