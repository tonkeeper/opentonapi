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

type BubbleUnSubscription struct {
	Subscription, Subscriber, Beneficiary Account
	Success                               bool
}

var InitialSubscriptionStraw = Straw[BubbleSubscription]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.PaymentRequestResponseMsgOp), AmountInterval(1, 1<<62)},
	Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Subscriber = *tx.inputFrom
		newAction.Subscription = tx.account
		newAction.Amount = tx.inputAmount
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

var ExtendedSubscriptionStraw = Straw[BubbleSubscription]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.SubscriptionV1)},
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
			newAction.Amount = int64(request.Amount.Grams)
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
			Amount:       b.Amount,
			First:        b.First,
		},
		Success: true,
		Type:    Subscription,
	}
}

var UnSubscriptionBySubscriberStraw = Straw[BubbleUnSubscription]{
	CheckFuncs: []bubbleCheck{IsTx},
	Builder: func(newAction *BubbleUnSubscription, bubble *Bubble) error {
		newAction.Subscriber = bubble.Info.(BubbleTx).account
		newAction.Success = true
		return nil
	},
	Children: []Straw[BubbleUnSubscription]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOpcode(abi.WalletPluginDestructMsgOpCode)}, // TODO: or check subscription interfaces
			Builder: func(newAction *BubbleUnSubscription, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				newAction.Subscription = tx.account
				newAction.Success = newAction.Success && tx.success
				return nil
			},
			SingleChild: &Straw[BubbleUnSubscription]{
				Optional:   true,
				CheckFuncs: []bubbleCheck{IsTx, Or(HasOpcode(abi.WalletPluginDestructMsgOpCode), HasEmptyBody)},
				Builder: func(newAction *BubbleUnSubscription, bubble *Bubble) error {
					newAction.Beneficiary = bubble.Info.(BubbleTx).account
					return nil
				},
			},
		},
		{
			CheckFuncs: []bubbleCheck{Is(BubbleRemoveExtension{})},
			Builder: func(newAction *BubbleUnSubscription, bubble *Bubble) error {
				removeExtTx := bubble.Info.(BubbleRemoveExtension)
				if removeExtTx.Extension != newAction.Subscription.Address {
					newAction.Success = false
				}
				newAction.Success = newAction.Success && removeExtTx.Success
				return nil
			},
		},
	},
}

var UnSubscriptionByBeneficiaryOrExpiredStraw = Straw[BubbleUnSubscription]{
	CheckFuncs: []bubbleCheck{IsTx, Or(IsExternal, HasOpcode(abi.WalletPluginDestructMsgOpCode))}, // TODO: or check subscription interfaces
	Builder: func(newAction *BubbleUnSubscription, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Subscription = tx.account
		newAction.Success = tx.success
		if !tx.external && tx.inputFrom != nil {
			newAction.Beneficiary = *tx.inputFrom
		}
		return nil
	},
	Children: []Straw[BubbleUnSubscription]{
		{
			CheckFuncs: []bubbleCheck{IsTx}, // wallet here. Don't check opcodes, the only plugin deletion event is important
			Builder: func(newAction *BubbleUnSubscription, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				newAction.Subscriber = tx.account
				return nil
			},
			SingleChild: &Straw[BubbleUnSubscription]{
				CheckFuncs: []bubbleCheck{Is(BubbleRemoveExtension{})},
				Builder: func(newAction *BubbleUnSubscription, bubble *Bubble) error {
					removeExtTx := bubble.Info.(BubbleRemoveExtension)
					if removeExtTx.Extension != newAction.Subscription.Address {
						newAction.Success = false
					}
					newAction.Success = newAction.Success && removeExtTx.Success
					return nil
				},
			},
		},
		// TODO: can not detect newAction.Beneficiary here for external
		// TODO: We cannot match the two other messages because we cannot distinguish them.
		// TODO: fix matching
		//{
		//	// to beneficiary
		//	CheckFuncs: []bubbleCheck{IsTx, Or(HasOpcode(abi.WalletPluginDestructMsgOpCode), HasEmptyBody)},
		//},
		//{
		//	Optional:   true, // to reward address for external
		//	CheckFuncs: []bubbleCheck{IsTx, HasEmptyBody},
		//},
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
