package bath

import (
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo/abi"
	"math/big"
)

type BubbleSubscription struct {
	Subscription, Subscriber, Beneficiary Account    // TODO: Beneficiary not valid name for subscription V2 (recipient)
	Amount                                core.Price // TODO: replace with price
	Success                               bool
	First                                 bool
}

type BubbleUnSubscription struct {
	Subscription, Subscriber, Beneficiary Account
	Success                               bool
}

var InitialSubscriptionStraw = Straw[BubbleSubscription]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.SubscriptionV1), HasOperation(abi.PaymentRequestResponseMsgOp), AmountInterval(1, 1<<62)}, // TODO: equal to abi.SubscriptionV2PaymentConfirmedMsgOp
	Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Subscriber = *tx.inputFrom
		newAction.Subscription = tx.account
		newAction.Amount = core.Price{ // only ton payments supported by Subscription V1
			Type:   core.CurrencyTON,
			Amount: *big.NewInt(tx.inputAmount),
		}
		newAction.First = len(tx.init) != 0
		return nil
	},
	SingleChild: &Straw[BubbleSubscription]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.SubscriptionPaymentMsgOp)},
		Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
			newAction.Success = true
			newAction.Beneficiary = bubble.Info.(BubbleTx).account
			return nil
		},
	},
}

var InitialSubscriptionV2Straw = Straw[BubbleSubscription]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.SubscriptionV2), HasOperation(abi.SubscriptionV2PaymentConfirmedMsgOp), AmountInterval(1, 1<<62)}, // TODO: equal to abi.PaymentRequestResponseMsgOp
	Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Subscriber = *tx.inputFrom
		newAction.Subscription = tx.account
		newAction.Amount = core.Price{ // only ton payments supported by Subscription V2
			Type:   core.CurrencyTON,
			Amount: *big.NewInt(tx.inputAmount),
		}
		newAction.First = len(tx.init) != 0
		return nil
	},
	// TODO: how to check custom out msg?
	SingleChild: &Straw[BubbleSubscription]{
		CheckFuncs: []bubbleCheck{IsTx, Or(HasOperation(abi.TextCommentMsgOp), HasEmptyBody)},
		Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
			newAction.Success = true // subscription V2 uses non-bounce flag
			newAction.Beneficiary = bubble.Info.(BubbleTx).account
			return nil
		},
	},
}

// TODO: conflicts with ExtendedSubscriptionW5Straw
var ExtendedSubscriptionStraw = Straw[BubbleSubscription]{
	CheckFuncs: []bubbleCheck{IsTx, Or(HasInterface(abi.SubscriptionV1), HasInterface(abi.SubscriptionV2))}, // same behavior sub v1 and v2 for wallet V4
	Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Subscription = tx.account
		return nil
	},
	SingleChild: &Straw[BubbleSubscription]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.PaymentRequestMsgOp)},
		Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			request := tx.decodedBody.Value.(abi.PaymentRequestMsgBody)
			newAction.Subscriber = tx.account
			newAction.Success = tx.success
			newAction.Amount = core.Price{ // only ton payments supported by SubscriptionV1
				Type:   core.CurrencyTON,
				Amount: *big.NewInt(int64(request.Amount.Grams)),
			}
			return nil
		},
		SingleChild: &Straw[BubbleSubscription]{
			Optional:   true,
			CheckFuncs: []bubbleCheck{Is(BubbleSubscription{})},
			Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
				sub := bubble.Info.(BubbleSubscription)
				newAction.First = false
				newAction.Beneficiary = sub.Beneficiary
				newAction.Success = true
				return nil
			},
		},
	},
}

// TODO: deploy of subscription also may be first payment but has another pattern

var ExtendedSubscriptionW5Straw = Straw[BubbleSubscription]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.SubscriptionV2)},
	Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Subscription = tx.account
		return nil
	},
	SingleChild: &Straw[BubbleSubscription]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.WalletExtensionActionV5R1MsgOp), // TODO: check wallet interface?
			func(bubble *Bubble) bool {
				tx := bubble.Info.(BubbleTx)
				request := tx.decodedBody.Value.(abi.WalletExtensionActionV5R1MsgBody)
				if request.Actions == nil {
					return false
				}
				if len([]abi.W5SendMessageAction(*request.Actions)) != 1 {
					return false
				}
				return true
			}},
		Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			request := tx.decodedBody.Value.(abi.WalletExtensionActionV5R1MsgBody)
			newAction.Subscriber = tx.account
			newAction.Success = tx.success // TODO: if success but no out msgs? maybe check wallet interface?
			walletAction := []abi.W5SendMessageAction(*request.Actions)[0]
			newAction.Amount = core.Price{ // only ton payments supported by SubscriptionV2
				Type:   core.CurrencyTON,
				Amount: *big.NewInt(int64(walletAction.Msg.MessageInternal.Value.Grams)),
			}
			return nil
		},
		SingleChild: &Straw[BubbleSubscription]{
			Optional:   true,
			CheckFuncs: []bubbleCheck{Is(BubbleSubscription{})},
			Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
				sub := bubble.Info.(BubbleSubscription)
				newAction.First = false
				newAction.Beneficiary = sub.Beneficiary
				newAction.Success = true
				return nil
			},
		},
	},
}

func (b BubbleSubscription) ToAction() (action *Action) {
	return &Action{
		Subscription: &SubscriptionAction{
			Subscription: b.Subscription.Address,
			Subscriber:   b.Subscriber.Address,
			Beneficiary:  b.Beneficiary.Address,
			// Amount:       b.Amount, // TODO: replace with Price
			First: b.First,
		},
		Success: true,
		Type:    Subscription,
	}
}

var UnSubscriptionStraw = Straw[BubbleUnSubscription]{
	CheckFuncs: []bubbleCheck{IsTx},
	Builder: func(newAction *BubbleUnSubscription, bubble *Bubble) error {
		newAction.Subscriber = bubble.Info.(BubbleTx).account
		return nil
	},
	SingleChild: &Straw[BubbleUnSubscription]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.WalletPluginDestructMsgOp)},
		Builder: func(newAction *BubbleUnSubscription, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.Subscription = tx.account
			newAction.Success = tx.success
			return nil
		},
		SingleChild: &Straw[BubbleUnSubscription]{
			Optional:   true,
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.WalletPluginDestructMsgOp)},
			Builder: func(newAction *BubbleUnSubscription, bubble *Bubble) error {
				newAction.Beneficiary = bubble.Info.(BubbleTx).account
				return nil
			},
		},
	},
}

func (b BubbleUnSubscription) ToAction() (action *Action) {
	return &Action{
		UnSubscription: &UnSubscriptionAction{
			Subscription: b.Subscription.Address,
			Subscriber:   b.Subscriber.Address,
			Beneficiary:  b.Beneficiary.Address,
		},
		Success: true,
		Type:    UnSubscription,
	}
}
