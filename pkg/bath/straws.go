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
	JettonTransferClassicStraw,
	JettonTransferMinimalStraw,
	JettonBurnStraw,
	WtonMintStraw, //10
	NftPurchaseStraw,
	StonfiSwapStraw,
	StonfiSwapV2Straw,
	UniversalDedustStraw{},
	TgAuctionV1InitialBidStraw, //15
	StrawAuctionBigGetgems,
	StrawAuctionBuyGetgems,
	StrawAuctionBuyFragments,
	JettonMintFromMasterStraw,
	JettonMintStrawGovernance,
	InvoicePaymentStrawNative,
	InvoicePaymentStrawJetton,
	MegatonFiJettonSwap,
	UnSubscriptionBySubscriberStraw,
	UnSubscriptionByBeneficiaryOrExpiredStraw,
	SubscriptionDeployStraw,
	SubscriptionPaymentStraw,
	SubscriptionPaymentWithRequestFundsStraw,
	DepositLiquidStakeStraw,
	OldPendingWithdrawRequestLiquidStraw,
	PendingWithdrawRequestLiquidStraw,
	ElectionsDepositStakeStraw,
	ElectionsRecoverStakeStraw,
	DepositTFStakeStraw,
	WithdrawTFStakeRequestStraw,
	WithdrawStakeImmediatelyStraw,
	WithdrawLiquidStake,
	DNSRenewStraw,
	//DepositEthenaStakeStraw,
	//WithdrawEthenaStakeRequestStraw,
	BidaskSwapStraw,
	BidaskSwapStrawReverse,
	BidaskJettonSwapStraw,
	MooncxSwapStraw,
	MooncxSwapStrawReverse,
	ToncoSwapStraw,
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
