package bath

import (
	"errors"
	"github.com/google/uuid"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"math/big"
)

type BubbleInvoicePayment struct {
	Sender, Recipient tongo.AccountID
	InvoiceID         uuid.UUID
	Amount            big.Int
	Jetton            *tongo.AccountID
	CurrencyID        *int32
	Success           bool
}

func (b BubbleInvoicePayment) ToAction() (action *Action) {
	return &Action{
		InvoicePayment: &InvoicePaymentAction{
			Sender:     b.Sender,
			Recipient:  b.Recipient,
			InvoiceID:  b.InvoiceID,
			Amount:     b.Amount,
			Jetton:     b.Jetton,
			CurrencyID: b.CurrencyID,
		},
		Success: b.Success,
		Type:    InvoicePayment,
	}
}

var InvoicePaymentStrawNative = Straw[BubbleInvoicePayment]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.InvoicePayloadMsgOp), AmountInterval(1, 1<<62)},
	Builder: func(newAction *BubbleInvoicePayment, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		invoicePayload := tx.decodedBody.Value.(abi.InvoicePayloadMsgBody)
		id, err := uuid.FromBytes(invoicePayload.Id[:])
		if err != nil {
			return err
		}
		newAction.InvoiceID = id
		newAction.Amount = *big.NewInt(tx.inputAmount)
		for c, am := range tx.inputExtraAmount {
			newAction.Amount = big.Int(am)
			newAction.CurrencyID = &c
			break
		}
		if tx.inputFrom == nil {
			return errors.New("empty input")
		}
		newAction.Sender = tx.inputFrom.Address
		newAction.Recipient = tx.account.Address
		newAction.Success = !tx.bounced // TODO: or success?
		return nil
	},
}

var InvoicePaymentStrawJetton = Straw[BubbleInvoicePayment]{
	CheckFuncs: []bubbleCheck{func(bubble *Bubble) bool {
		jettonTx, ok := bubble.Info.(BubbleJettonTransfer)
		if !ok {
			return false
		}
		if jettonTx.sender == nil || jettonTx.recipient == nil {
			return false
		}
		if jettonTx.payload.SumType != abi.InvoicePayloadJettonOp {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleInvoicePayment, bubble *Bubble) error {
		jettonTx := bubble.Info.(BubbleJettonTransfer)
		// type already checked
		invoicePayload := jettonTx.payload.Value.(abi.InvoicePayloadJettonPayload)
		id, err := uuid.FromBytes(invoicePayload.Id[:])
		if err != nil {
			return err
		}
		newAction.InvoiceID = id
		newAction.Amount = big.Int(jettonTx.amount)
		newAction.Jetton = &jettonTx.master
		// sender and recipient already checked for nil
		newAction.Sender = jettonTx.sender.Address
		newAction.Recipient = jettonTx.recipient.Address
		newAction.Success = jettonTx.success
		return nil
	},
}
