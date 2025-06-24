package bath

import (
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"math/big"
)

type BubbleSubscription struct {
	Subscription, Subscriber tongo.AccountID
	Beneficiary, WithdrawTo  tongo.AccountID
	Amount                   int64
	Success                  bool
	First                    bool
}

func (b BubbleSubscription) ToAction() (action *Action) {
	return &Action{
		Subscribe: &SubscribeAction{
			Subscription: b.Subscription,
			Subscriber:   b.Subscriber,
			Beneficiary:  b.Beneficiary,
			WithdrawTo:   b.WithdrawTo,
			Amount: core.Price{
				Type:   core.CurrencyTON,
				Amount: *big.NewInt(b.Amount)},
			First: b.First,
		},
		Success: b.Success,
		Type:    Subscribe,
	}
}

type BubbleUnSubscription struct {
	Subscription, Subscriber tongo.AccountID
	Beneficiary              tongo.AccountID // beneficiary != withdraw_to address for subscription V2
	Success                  bool
}

func (b BubbleUnSubscription) ToAction() (action *Action) {
	return &Action{
		UnSubscribe: &UnSubscribeAction{
			Subscription: b.Subscription,
			Subscriber:   b.Subscriber,
			Beneficiary:  b.Beneficiary,
		},
		Success: b.Success,
		Type:    UnSubscribe,
	}
}

var SubscriptionDeployStraw = Straw[BubbleSubscription]{
	CheckFuncs: []bubbleCheck{IsTx}, // Wallet. Starting from here to consume the addExtension bubble.
	Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Subscriber = tx.account.Address
		newAction.First = true
		newAction.Success = tx.success
		return nil
	},
	Children: []Straw[BubbleSubscription]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.SubscriptionDeployMsgOp)},
			Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				newAction.Subscription = tx.account.Address
				if tx.additionalInfo != nil && tx.additionalInfo.SubscriptionInfo != nil {
					newAction.Beneficiary = tx.additionalInfo.SubscriptionInfo.Beneficiary
					newAction.WithdrawTo = tx.additionalInfo.SubscriptionInfo.WithdrawTo
					if len(bubble.Children) > 1 { // specify the amount only if there was a payment.
						newAction.Amount = tx.additionalInfo.SubscriptionInfo.PaymentPerPeriod
					}
				}
				newAction.Success = newAction.Success && tx.success
				return nil
			},
			Children: []Straw[BubbleSubscription]{
				{
					CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
					Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
						newAction.Success = newAction.Success && bubble.Info.(BubbleContractDeploy).Success
						return nil
					},
				},
				// initial payment - bubble with arbitrary payload is not consumed.
			},
		},
		{
			CheckFuncs: []bubbleCheck{Is(BubbleAddExtension{})},
			Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
				addExtTx := bubble.Info.(BubbleAddExtension)
				//if addExtTx.Extension != newAction.Subscription { // TODO: Can't verify because the subscription address is not yet filled in.
				//	newAction.Success = false
				//}
				newAction.Success = newAction.Success && addExtTx.Success
				return nil
			},
		},
	},
}

var SubscriptionPaymentStraw = Straw[BubbleSubscription]{
	CheckFuncs: []bubbleCheck{
		IsTx,
		Or(HasInterface(abi.SubscriptionV1), HasInterface(abi.SubscriptionV2)), // define interfaces because the opcode is too generic
		HasOperation(abi.PaymentConfirmedMsgOp),
		AmountInterval(1, 1<<62)},
	Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Subscription = tx.account.Address
		if tx.additionalInfo != nil && tx.additionalInfo.SubscriptionInfo != nil {
			newAction.Subscriber = tx.additionalInfo.SubscriptionInfo.Wallet
			newAction.Beneficiary = tx.additionalInfo.SubscriptionInfo.Beneficiary
			newAction.WithdrawTo = tx.additionalInfo.SubscriptionInfo.WithdrawTo
			newAction.Amount = tx.additionalInfo.SubscriptionInfo.PaymentPerPeriod
		}
		if len(bubble.Children) > 0 { // optional reward + withdraw to
			newAction.Success = tx.success
		}
		return nil
	},
	// optional reward is not consumed
	// withdraw to - bubble with arbitrary payload is not consumed
}

var SubscriptionPaymentWithRequestFundsStraw = Straw[BubbleSubscription]{
	CheckFuncs: []bubbleCheck{
		IsTx,
		Or(HasInterface(abi.SubscriptionV1), HasInterface(abi.SubscriptionV2))},
	Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Subscription = tx.account.Address
		if tx.additionalInfo != nil && tx.additionalInfo.SubscriptionInfo != nil {
			newAction.Subscriber = tx.additionalInfo.SubscriptionInfo.Wallet
			newAction.Beneficiary = tx.additionalInfo.SubscriptionInfo.Beneficiary
			newAction.WithdrawTo = tx.additionalInfo.SubscriptionInfo.WithdrawTo
			newAction.Amount = tx.additionalInfo.SubscriptionInfo.PaymentPerPeriod
		}
		newAction.First = false
		newAction.Success = false
		return nil
	},
	SingleChild: &Straw[BubbleSubscription]{
		CheckFuncs: []bubbleCheck{
			IsTx,
			Or(HasOperation(abi.PaymentRequestMsgOp), HasOperation(abi.WalletExtensionActionV5R1MsgOp)),
			func(bubble *Bubble) bool {
				if body, ok := bubble.Info.(BubbleTx).decodedBody.Value.(abi.WalletExtensionActionV5R1MsgBody); ok {
					return body.Extended == nil || len(*body.Extended) == 0 // exclude self destroy
				}
				return true
			},
		},
		SingleChild: &Straw[BubbleSubscription]{
			Optional:   true,
			CheckFuncs: []bubbleCheck{Is(BubbleSubscription{})},
			Builder: func(newAction *BubbleSubscription, bubble *Bubble) error {
				sub := bubble.Info.(BubbleSubscription)
				newAction.Success = sub.Success
				return nil
			},
		},
	},
}

var UnSubscriptionBySubscriberStraw = Straw[BubbleUnSubscription]{
	CheckFuncs: []bubbleCheck{IsTx}, // Wallet. Starting from here to consume the removeExtension bubble.
	Builder: func(newAction *BubbleUnSubscription, bubble *Bubble) error {
		newAction.Success = true
		return nil
	},
	Children: []Straw[BubbleUnSubscription]{
		{
			CheckFuncs: []bubbleCheck{
				IsTx,
				HasOperation(abi.WalletPluginDestructMsgOp),
				Or(HasInterface(abi.SubscriptionV1), HasInterface(abi.SubscriptionV2)),
			},
			Builder: func(newAction *BubbleUnSubscription, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				newAction.Subscription = tx.account.Address
				if tx.additionalInfo != nil && tx.additionalInfo.SubscriptionInfo != nil {
					newAction.Subscriber = tx.additionalInfo.SubscriptionInfo.Wallet
					newAction.Beneficiary = tx.additionalInfo.SubscriptionInfo.Beneficiary
				}
				newAction.Success = newAction.Success && tx.success
				return nil
			},
		},
		{
			// TODO: Set to failed if there was no plugin removal action
			CheckFuncs: []bubbleCheck{Is(BubbleRemoveExtension{})},
			Builder: func(newAction *BubbleUnSubscription, bubble *Bubble) error {
				removeExtTx := bubble.Info.(BubbleRemoveExtension)
				if removeExtTx.Extension != newAction.Subscription {
					newAction.Success = false
				}
				newAction.Success = newAction.Success && removeExtTx.Success
				return nil
			},
		},
	},
}

var UnSubscriptionByBeneficiaryOrExpiredStraw = Straw[BubbleUnSubscription]{
	CheckFuncs: []bubbleCheck{
		IsTx,
		Or(IsExternal, HasOperation(abi.WalletPluginDestructMsgOp)),
		Or(HasInterface(abi.SubscriptionV1), HasInterface(abi.SubscriptionV2)),
	},
	Builder: func(newAction *BubbleUnSubscription, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Subscription = tx.account.Address
		newAction.Success = tx.success
		if tx.additionalInfo != nil && tx.additionalInfo.SubscriptionInfo != nil {
			newAction.Subscriber = tx.additionalInfo.SubscriptionInfo.Wallet
			newAction.Beneficiary = tx.additionalInfo.SubscriptionInfo.Beneficiary
		}
		return nil
	},
	Children: []Straw[BubbleUnSubscription]{
		{
			CheckFuncs: []bubbleCheck{IsTx}, // wallet. Don't check opcodes, the only plugin deletion event is important
			Builder: func(newAction *BubbleUnSubscription, bubble *Bubble) error {
				newAction.Success = newAction.Success && bubble.Info.(BubbleTx).success
				return nil
			},
			SingleChild: &Straw[BubbleUnSubscription]{
				CheckFuncs: []bubbleCheck{Is(BubbleRemoveExtension{})},
				Builder: func(newAction *BubbleUnSubscription, bubble *Bubble) error {
					removeExtTx := bubble.Info.(BubbleRemoveExtension)
					if removeExtTx.Extension != newAction.Subscription {
						newAction.Success = false
					}
					newAction.Success = newAction.Success && removeExtTx.Success
					return nil
				},
			},
		},
		// optional reward is not consumed
		// withdraw to beneficiary is not consumed
	},
}
