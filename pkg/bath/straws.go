package bath

import (
	"math/big"

	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/ton"
)

type Merger interface {
	// Merge extracts information from the given bubble and its children and modifies the bubble if needed.
	// If the bubble is modified this function return true.
	Merge(bubble *Bubble) (success bool)
}

var JettonTransfersBurnsMints = []Merger{
	StonfiV1PTONStraw,
	StonfiV2PTONStrawReverse,
	StonfiV2PTONStraw,
	FlawedJettonTransferClassicStraw,
	FlawedJettonTransferMinimalStraw,
	JettonTransferClassicStraw,
	JettonTransferMinimalStraw,
	JettonBurnStraw,
	JettonMintFromMasterStraw,
	JettonMintStrawGovernance,
	WtonMintStraw,
}

var NFTStraws = []Merger{
	NftTransferStraw,
	NftTransferNotifyStraw,
}

var DefaultStraws = []Merger{
	StrawFindAuctionBidFragmentSimple, //0
	GasRelayerStraw,
	NftTransferStraw,
	NftTransferNotifyStraw,
	StonfiV1PTONStraw,
	StonfiV2PTONStrawReverse, //5
	StonfiV2PTONStraw,
	FlawedJettonTransferClassicStraw,
	FlawedJettonTransferMinimalStraw,
	JettonTransferClassicStraw,
	JettonTransferMinimalStraw, // 10
	JettonBurnStraw,
	WtonMintStraw,
	NftPurchaseStraw,
	StonfiSwapStraw,
	StonfiSwapV2Straw, // 15
	UniversalDedustStraw{},
	TgAuctionV1InitialBidStraw,
	StrawAuctionBigGetgems,
	StrawAuctionBuyGetgems,
	StrawAuctionBuyFragments, // 20
	JettonMintFromMasterStraw,
	JettonMintStrawGovernance,
	InvoicePaymentStrawNative,
	InvoicePaymentStrawJetton,
	MegatonFiJettonSwap, // 25
	UnSubscriptionBySubscriberStraw,
	UnSubscriptionByBeneficiaryOrExpiredStraw,
	SubscriptionDeployStraw,
	SubscriptionPaymentStraw,
	SubscriptionPaymentWithRequestFundsStraw, // 30
	DepositLiquidStakeStraw,
	OldPendingWithdrawRequestLiquidStraw,
	PendingWithdrawRequestLiquidStraw,
	ElectionsDepositStakeStraw,
	ElectionsRecoverStakeStraw, // 35
	DepositTFStakeStraw,
	WithdrawTFStakeRequestStraw,
	WithdrawStakeImmediatelyStraw,
	WithdrawLiquidStake,
	DNSRenewStraw, // 40
	BidaskLiquidityDepositBothNativeStraw,
	BidaskLiquidityDepositBothJettonStraw,
	BidaskLiquidityDepositJettonStraw,
	StonfiLiquidityDepositSingle,
	StonfiLiquidityDepositBoth, // 45
	DepositEthenaStakeStraw,
	WithdrawEthenaStakeRequestStraw,
	BidaskSwapStraw,
	BidaskSwapStrawReverse,
	BidaskJettonSwapStraw, // 50
	MooncxSwapStraw,
	MooncxSwapStrawReverse,
	MoocxLiquidityDepositJettonStraw,
	MoocxLiquidityDepositNativeStraw,
	MoocxLiquidityDepositBothStraw, // 55
	ToncoSwapStraw,
	ToncoDepositLiquiditySingleStraw,
	ToncoDepositLiquidityBothStraw,
	ToncoDepositLiquidityWithRefundStraw,
	DepositAffluentEarnStraw, // 60
	DepositAffluentEarnWithOraclesStraw,
	WithdrawAffluentEarnRequestStraw,
	InstantWithdrawAffluentEarnStraw,
	InstantWithdrawAffluentEarnWithOraclesStraw,
}

var JettonTransferClassicStraw = Straw[BubbleJettonTransfer]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.JettonWallet), HasOperation(abi.JettonTransferMsgOp)},
	Builder: func(newAction *BubbleJettonTransfer, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.master, _ = tx.additionalInfo.JettonMaster(tx.account.Address)
		newAction.senderWallet = tx.account.Address
		newAction.sender = tx.inputFrom
		body := tx.decodedBody.Value.(abi.JettonTransferMsgBody)
		newAction.amount = body.Amount
		newAction.payload = body.ForwardPayload.Value
		recipient, err := ton.AccountIDFromTlb(body.Destination)
		if err == nil && recipient != nil {
			newAction.recipient = &Account{Address: *recipient}
			bubble.Accounts = append(bubble.Accounts, *recipient)
		}
		return nil
	},
	SingleChild: &Straw[BubbleJettonTransfer]{
		CheckFuncs: []bubbleCheck{IsTx, Or(HasInterface(abi.JettonWallet), HasOperation(abi.JettonInternalTransferMsgOp))}, //todo: remove Or(). both should be in new indexer
		Optional:   true,
		Builder: func(newAction *BubbleJettonTransfer, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.recipientWallet = tx.account.Address
			if newAction.master.IsZero() {
				newAction.master, _ = tx.additionalInfo.JettonMaster(tx.account.Address)
			}
			newAction.success = tx.success
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

var FlawedJettonTransferClassicStraw = Straw[BubbleFlawedJettonTransfer]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.JettonWallet), HasOperation(abi.JettonTransferMsgOp), func(bubble *Bubble) bool {
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
		body := tx.decodedBody.Value.(abi.JettonTransferMsgBody)
		newAction.sentAmount = body.Amount
		newAction.payload = body.ForwardPayload.Value
		recipient, err := ton.AccountIDFromTlb(body.Destination)
		if err == nil && recipient != nil {
			newAction.recipient = &Account{Address: *recipient}
			bubble.Accounts = append(bubble.Accounts, *recipient)
		}
		return nil
	},
	SingleChild: &Straw[BubbleFlawedJettonTransfer]{
		CheckFuncs: []bubbleCheck{IsTx, Or(HasInterface(abi.JettonWallet), HasOperation(abi.JettonInternalTransferMsgOp))}, //todo: remove Or(). both should be in new indexer
		Optional:   true,
		Builder: func(newAction *BubbleFlawedJettonTransfer, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.recipientWallet = tx.account.Address
			if newAction.master.IsZero() {
				newAction.master, _ = tx.additionalInfo.JettonMaster(tx.account.Address)
			}
			body, _ := tx.decodedBody.Value.(abi.JettonInternalTransferMsgBody)
			newAction.receivedAmount = body.Amount
			newAction.success = tx.success
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
