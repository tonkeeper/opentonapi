package bath

import (
	"github.com/tonkeeper/tongo/abi"
)

type BubbleSubscription struct {
	Subscription, Subscriber, WithdrawTo Account
	Amount                               int64
	Success                              bool
	First                                bool
}

type BubbleUnSubscription struct {
	Subscription, Subscriber Account
	Beneficiary              Account // TODO: better to remove. beneficicar != WithdrawTo and we cannot always determine it
	Success                  bool
}

var SubscriptionDeployStraw = Straw[BubbleSubscription]{
	CheckFuncs: []bubbleCheck{IsTx}, // wallet
	Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
		newAction.Subscriber = bubble.Info.(BubbleTx).account
		newAction.Success = true
		return nil
	},
	Children: []Straw[BubbleSubscription]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOpcode(abi.SubscriptionV2DeployMsgOpCode)},
			Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				newAction.Subscriber = *tx.inputFrom
				newAction.Subscription = tx.account
				newAction.First = true
				if len(bubble.Children) == 2 { // 0 - withdraw, 1 - deploy. if len == 1 - only deploy
					tx, ok := bubble.Children[0].Info.(BubbleTx)
					if ok {
						newAction.WithdrawTo = tx.account
						newAction.Amount = tx.inputAmount
					}
				}
				return nil
			},
			Children: []Straw[BubbleSubscription]{
				{
					CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
					Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
						deployTx := bubble.Info.(BubbleContractDeploy)
						newAction.Success = newAction.Success && deployTx.Success
						return nil
					},
				},
				// TODO: won't consume a bubble with arbitrary payload
				//{
				//	Optional:   true, // to reward address for external
				//	CheckFuncs: []bubbleCheck{IsTx},
				//	Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
				//		tx := bubble.Info.(BubbleTx)
				//		newAction.Amount = tx.inputAmount
				//		return nil
				//	},
				//},
			},
		},
		{
			CheckFuncs: []bubbleCheck{Is(BubbleAddExtension{})},
			Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
				addExtTx := bubble.Info.(BubbleAddExtension)
				if addExtTx.Extension != newAction.Subscription.Address {
					newAction.Success = false
				}
				newAction.Success = newAction.Success && addExtTx.Success
				return nil
			},
		},
	},
}

var InitialSubscriptionStraw = Straw[BubbleSubscription]{
	CheckFuncs: []bubbleCheck{IsTx, HasOpcode(abi.SubscriptionV2PaymentConfirmedMsgOpCode), AmountInterval(1, 1<<62)},
	Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Subscriber = *tx.inputFrom
		newAction.Subscription = tx.account
		newAction.Amount = tx.inputAmount
		switch len(bubble.Children) {
		case 1: // msg to withdraw to
			if tx, ok := bubble.Children[0].Info.(BubbleTx); ok {
				newAction.WithdrawTo = tx.account
				newAction.Success = true
			}
		case 2: // reward + msg to withdraw to
			if tx, ok := bubble.Children[1].Info.(BubbleTx); ok {
				newAction.WithdrawTo = tx.account
				newAction.Success = true
			}
		}
		return nil
	},
	// TODO: We cannot match the two other messages because we cannot distinguish them.
	// TODO: fix matching
	//SingleChild: &Straw[BubbleSubscription]{
	//	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.SubscriptionPaymentMsgOp)},
	//	Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
	//		newAction.Success = true
	//		newAction.WithdrawTo = bubble.Info.(BubbleTx).account
	//		return nil
	//	},
	//},
}

var ExtendedSubscriptionStraw = Straw[BubbleSubscription]{
	CheckFuncs: []bubbleCheck{IsTx, Or(HasInterface(abi.SubscriptionV1), HasInterface(abi.SubscriptionV2))},
	Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Subscription = tx.account
		return nil
	},
	SingleChild: &Straw[BubbleSubscription]{
		CheckFuncs: []bubbleCheck{IsTx, Or(HasOperation(abi.PaymentRequestMsgOp), HasOperation(abi.WalletExtensionActionV5R1MsgOp))},
		Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			if request, ok := tx.decodedBody.Value.(abi.PaymentRequestMsgBody); ok {
				newAction.Amount = int64(request.Amount.Grams)
			}
			// TODO: need to decode amount from WalletExtensionAction
			newAction.Subscriber = tx.account
			newAction.Success = false
			return nil
		},
		SingleChild: &Straw[BubbleSubscription]{
			Optional:   true,
			CheckFuncs: []bubbleCheck{Is(BubbleSubscription{})},
			Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
				sub := bubble.Info.(BubbleSubscription)
				newAction.First = false
				newAction.WithdrawTo = sub.WithdrawTo
				newAction.Amount = sub.Amount
				newAction.Success = true
				return nil
			},
		},
	},
}

func (b BubbleSubscription) ToAction() (action *Action) {
	return &Action{
		Subscribe: &SubscribeAction{
			Subscription: b.Subscription.Address,
			Subscriber:   b.Subscriber.Address,
			Beneficiary:  b.WithdrawTo.Address,
			Amount:       b.Amount,
			First:        b.First,
		},
		Success: true,
		Type:    Subscribe,
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
		switch len(bubble.Children) {
		case 2: // remove + msg to beneficicar
			if tx, ok := bubble.Children[1].Info.(BubbleTx); ok {
				newAction.Beneficiary = tx.account
			}
		case 3: // remove + caller reward + msg to beneficicar
			if tx, ok := bubble.Children[2].Info.(BubbleTx); ok {
				newAction.Beneficiary = tx.account
			}
		}
		return nil
	},
	Children: []Straw[BubbleUnSubscription]{
		{
			Optional:   true, // to reward address
			CheckFuncs: []bubbleCheck{IsTx, HasEmptyBody},
		},
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
		{
			// to beneficiary
			CheckFuncs: []bubbleCheck{IsTx, Or(HasOpcode(abi.WalletPluginDestructMsgOpCode), HasEmptyBody)},
		},
	},
}

func (b BubbleUnSubscription) ToAction() (action *Action) {
	return &Action{
		UnSubscribe: &UnSubscribeAction{
			Subscription: b.Subscription.Address,
			Subscriber:   b.Subscriber.Address,
			Beneficiary:  b.Beneficiary.Address,
		},
		Success: true,
		Type:    UnSubscribe,
	}
}
