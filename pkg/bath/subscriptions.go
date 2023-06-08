package bath

import (
	"github.com/tonkeeper/tongo/abi"
)

type BubbleSubscription struct {
	Subscription, Subscriber, Beneficiary Account
	Amount                                int64
	Success                               bool
	First                                 bool
}

func FindInitialSubscription(bubble *Bubble) bool {
	txBubble, ok := bubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	if !txBubble.operation(abi.PaymentRequestResponseMsgOp) {
		return false
	}
	newBubble := Bubble{
		Accounts:  append(bubble.Accounts, txBubble.account.Address),
		ValueFlow: bubble.ValueFlow,
	}
	var beneficiary, subscriber Account
	if txBubble.inputFrom != nil {
		subscriber = *txBubble.inputFrom
	}
	var success bool
	newBubble.Children = ProcessChildren(bubble.Children,
		func(child *Bubble) *Merge {
			tx, ok := child.Info.(BubbleTx)
			if !ok {
				return nil
			}
			if !tx.operation(abi.SubscriptionPaymentMsgOp) {
				return nil
			}
			success = true
			beneficiary = tx.account
			newBubble.ValueFlow.Merge(child.ValueFlow)
			newBubble.Accounts = append(newBubble.Accounts, tx.account.Address)
			return &Merge{children: child.Children}
		})
	newBubble.Info = BubbleSubscription{
		Subscriber:   subscriber,
		Subscription: txBubble.account,
		Beneficiary:  beneficiary,
		Amount:       txBubble.inputAmount,
		Success:      success,
		First:        len(txBubble.init) != 0,
	}
	*bubble = newBubble
	return true
}

func FindExtendedSubscription(bubble *Bubble) bool {
	txBubble, ok := bubble.Info.(BubbleTx)
	if !ok ||
		!txBubble.account.Is(abi.Subscription) ||
		len(bubble.Children) != 1 ||
		len(bubble.Children[0].Children) != 1 {
		return false
	}
	commandBubble := bubble.Children[0]
	command, ok := commandBubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	subscriptionBubble := commandBubble.Children[0]
	subscription, ok := subscriptionBubble.Info.(BubbleSubscription)
	if !ok {
		return false
	}
	if !command.operation(abi.PaymentRequestMsgOp) {
		return false
	}
	request := command.decodedBody.Value.(abi.PaymentRequestMsgBody)
	subscription.Amount = int64(request.Amount.Grams)
	subscription.First = false
	subscriptionBubble.ValueFlow.Merge(bubble.ValueFlow)
	subscriptionBubble.ValueFlow.Merge(commandBubble.ValueFlow)
	subscriptionBubble.Info = subscription
	*bubble = *subscriptionBubble
	return false
}

func (b BubbleSubscription) ToAction() (action *Action) {
	return &Action{
		Subscription: &SubscriptionAction{
			Subscription: b.Subscription.Address,
			Subscriber:   b.Subscriber.Address,
			Beneficiary:  b.Beneficiary.Address,
			Amount:       b.Amount,
			First:        b.First,
		},
		Success: true,
		Type:    Subscription,
	}
}
