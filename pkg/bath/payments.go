package bath

import (
	"errors"
	"math/big"

	"github.com/google/uuid"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	abiXtr "github.com/tonkeeper/tongo/abi-tolk/abiGenerated/xtr"
	"github.com/tonkeeper/tongo/tlb"
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
			Currency: core.Currency{Type: core.CurrencyNative},
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

type BubbleDepositXTR struct {
	Recipient    tongo.AccountID
	JettonMaster tongo.AccountID
	Amount       big.Int
	Success      bool
}

func (b BubbleDepositXTR) ToAction() (action *Action) {
	return &Action{
		DepositXTR: &DepositXTRAction{
			Recipient:    b.Recipient,
			JettonMaster: b.JettonMaster,
			Amount:       b.Amount,
		},
		Success: b.Success,
		Type:    DepositXTR,
	}
}

var XTRDepositAction = Straw[BubbleDepositXTR]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abiXtr.XtrUpdatePaymentMsgOp)},
	Builder: func(newAction *BubbleDepositXTR, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.JettonMaster = tx.account.Address
		return nil
	},
	SingleChild: &Straw[BubbleDepositXTR]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abiXtr.XtrUpdateContractAndProcessMessageMsgOp)},
		SingleChild: &Straw[BubbleDepositXTR]{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abiXtr.XtrUpdateUserMsgOp)},
			SingleChild: &Straw[BubbleDepositXTR]{
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abiXtr.XtrUpdateContractAndProcessMessageMsgOp), func(bubble *Bubble) bool {
					tx := bubble.Info.(BubbleTx)
					body, ok := tx.decodedBody.Value.(*abiXtr.UpdateContractAndProcessMessage)
					if !ok {
						return false
					}
					var innerBody abiXtr.CommitXTR
					if err := innerBody.UnmarshalTLB(body.Payload.CopyCell(), nil); err != nil {
						return false
					}
					return true
				}},
				Builder: func(newAction *BubbleDepositXTR, bubble *Bubble) error {
					tx := bubble.Info.(BubbleTx)
					body := tx.decodedBody.Value.(*abiXtr.UpdateContractAndProcessMessage)
					var innerBody abiXtr.CommitXTR
					tlb.Unmarshal(body.Payload.CopyCell(), &innerBody)
					amount := new(big.Int).SetUint64(uint64(innerBody.Amount))
					newAction.Amount = *amount
					newAction.Recipient = tx.account.Address
					return nil
				},
				SingleChild: &Straw[BubbleDepositXTR]{
					CheckFuncs: []bubbleCheck{IsTx, HasOpcode(0xd53276db)},
					Builder: func(newAction *BubbleDepositXTR, bubble *Bubble) error {
						newAction.Success = true
						return nil
					},
				},
			},
		},
	},
}

type BubbleWithdrawXTR struct {
	User         tongo.AccountID
	JettonMaster tongo.AccountID
	Amount       big.Int
	Success      bool
}

func (b BubbleWithdrawXTR) ToAction() (action *Action) {
	return &Action{
		WithdrawXTR: &WithdrawXTRAction{
			User:         b.User,
			JettonMaster: b.JettonMaster,
			Amount:       b.Amount,
		},
		Success: b.Success,
		Type:    WithdrawXTR,
	}
}

var XTRWithdrawAction = Straw[BubbleWithdrawXTR]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonBurnMsgOp)},
	Builder: func(newAction *BubbleWithdrawXTR, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		body := tx.decodedBody.Value.(abi.JettonBurnMsgBody)
		newAction.Amount = big.Int(body.Amount)
		newAction.User = tx.account.Address
		return nil
	},
	SingleChild: &Straw[BubbleWithdrawXTR]{
		CheckFuncs: []bubbleCheck{IsTx, HasOpcode(0x7bdd97de), HasInterface(abi.XtrMaster)},
		SingleChild: &Straw[BubbleWithdrawXTR]{
			CheckFuncs: []bubbleCheck{IsTx, HasOpcode(0xd53276db)},
			Builder: func(newAction *BubbleWithdrawXTR, bubble *Bubble) error {
				newAction.Success = true
				return nil
			},
		},
	},
}
