package bath

import (
	"fmt"
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

var InvoicePaymentStraw = Straw[BubbleInvoicePayment]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.InvoicePayloadMsgOp), AmountInterval(1, 1<<62)},
	// TODO: detect extra currency and jettons
	Builder: func(newAction *BubbleInvoicePayment, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		invoicePayload := tx.decodedBody.Value.(abi.InvoicePayloadMsgBody)
		id, err := uuid.FromBytes(invoicePayload.Id[:])
		if err != nil {
			return err
		}
		newAction.InvoiceID = id
		newAction.Amount = *big.NewInt(tx.inputAmount)
		if tx.inputFrom == nil {
			// TODO: err?
			return fmt.Errorf("empty input")
		}
		newAction.Sender = tx.inputFrom.Address
		newAction.Recipient = tx.account.Address
		newAction.Success = tx.success
		return nil
	},
}
