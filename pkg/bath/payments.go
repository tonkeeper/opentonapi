package bath

import (
	"errors"
	"github.com/google/uuid"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"math/big"
)

type BubbleInvoicePayment struct {
	Sender, Recipient tongo.AccountID
	InvoiceID         uuid.UUID
	Price             core.Price
	Success           bool
}

func (b BubbleInvoicePayment) ToAction() (action *Action) {
	return &Action{
		Purchase: &PurchaseAction{
			Source:      b.Sender,
			Destination: b.Recipient,
			InvoiceID:   b.InvoiceID,
			Price:       b.Price,
		},
		Success: b.Success,
		Type:    Purchase,
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
		price := core.Price{
			Currency: core.Currency{Type: core.CurrencyTON},
			Amount:   *big.NewInt(tx.inputAmount),
		}
		for c, am := range tx.inputExtraAmount {
			price.Currency = core.Currency{
				Type:       core.CurrencyExtra,
				CurrencyID: &c,
			}
			price.Amount = big.Int(am)
			break
		}
		if tx.inputFrom == nil {
			return errors.New("empty input")
		}
		newAction.Sender = tx.inputFrom.Address
		newAction.Recipient = tx.account.Address
		newAction.Price = price
		newAction.Success = tx.success // TODO: or not bounced?
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
		newAction.Price = core.Price{
			Currency: core.Currency{
				Type:   core.CurrencyJetton,
				Jetton: &jettonTx.master,
			},
			Amount: big.Int(jettonTx.amount),
		}
		// sender and recipient already checked for nil
		newAction.Sender = jettonTx.sender.Address
		newAction.Recipient = jettonTx.recipient.Address
		newAction.Success = jettonTx.success
		return nil
	},
}
