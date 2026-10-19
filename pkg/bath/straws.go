package bath

import (
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/core"
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
	EthenaTsUSDeTransferStraw,
	FlawedJettonTransferClassicStraw,
	FlawedJettonTransferMinimalStraw,
	JettonTransferClassicStraw,
	JettonTransferMinimalStraw,
	// Before JettonBurnStraw, which names no owner for it without additional info.
	JettonBurnHipoUnstakeAllStraw,
	JettonBurnStraw,
	JettonMintFromMasterStraw,
	JettonMintStrawGovernance,
	WtonMintStraw,
	// hGRAM is minted with Hipo's own op.
	JettonMintHipoStraw,
	JettonMintHipoRollbackStraw,
}

var NFTStraws = []Merger{
	NftTransferStraw,
	NftTransferNotifyStraw,
}

func DefaultStraws(book AddressBook, infoSource core.InformationSource) []Merger {
	return []Merger{
		//0
		StrawFindAuctionBidFragmentSimple,
		NftTransferStraw,
		NftTransferNotifyStraw,
		StonfiV1PTONStraw,
		StonfiV2PTONStrawReverse,
		//5
		StonfiV2PTONStraw,
		EthenaTsUSDeTransferStraw,
		FlawedJettonTransferClassicStraw,
		FlawedJettonTransferMinimalStraw,
		XTRWithdrawAction,
		// 10
		JettonTransferClassicStraw,
		JettonTransferMinimalStraw,
		GasRelayerStraw(book),
		// Before JettonBurnStraw, which names no owner for it without additional info.
		JettonBurnHipoUnstakeAllStraw,
		JettonBurnStraw,
		// 15
		WtonMintStraw,
		NftPurchaseStraw,
		StonfiSwapStraw,
		UniversalStonfiStraw{},
		UniversalDedustStraw{},
		// 20
		TgAuctionV1InitialBidStraw,
		StrawAuctionBigGetgems,
		StrawAuctionBuyGetgems,
		StrawAuctionBuyFragments,
		JettonMintFromMasterStraw,
		// 25
		JettonMintStrawGovernance,
		InvoicePaymentStrawNative,
		InvoicePaymentStrawJetton,
		MegatonFiJettonSwap,
		UnSubscriptionBySubscriberStraw,
		// 30
		UnSubscriptionByBeneficiaryOrExpiredStraw,
		SubscriptionDeployStraw,
		SubscriptionPaymentStraw,
		SubscriptionPaymentWithRequestFundsStraw,
		DepositLiquidStakeStraw,
		// 35
		PendingWithdrawRequestLiquidStraw,
		ElectionsDepositStakeStraw,
		ElectionsRecoverStakeStraw,
		DepositTFStakeStraw,
		WithdrawTFStakeRequestStraw,
		// 40
		WithdrawStakeImmediatelyStraw,
		WithdrawLiquidStake,
		DNSRenewStraw,
		BidaskLiquidityDepositBothNativeStraw,
		BidaskLiquidityDepositBothJettonStraw,
		// 45
		BidaskLiquidityDepositJettonStraw,
		StonfiLiquidityDepositSingle,
		StonfiLiquidityDepositBoth,
		DepositEthenaStakeStraw,
		WithdrawEthenaStakeRequestStraw,
		// 50
		BidaskSwapStraw,
		BidaskSwapStrawReverse,
		BidaskJettonSwapStraw,
		MooncxSwapStraw,
		MooncxSwapStrawReverse,
		// 55
		MoocxLiquidityDepositJettonStraw,
		MoocxLiquidityDepositNativeStraw,
		MoocxLiquidityDepositBothStraw,
		ToncoSwapStraw,
		ToncoDepositLiquiditySingleStraw,
		// 60
		ToncoDepositLiquidityBothStraw,
		ToncoDepositLiquidityWithRefundStraw,
		DepositAffluentEarnStraw,
		DepositAffluentEarnWithOraclesStraw,
		WithdrawAffluentEarnRequestStraw,
		// 65
		InstantWithdrawAffluentEarnStraw,
		InstantWithdrawAffluentEarnWithOraclesStraw,
		PythOraclePriceUpdateStraw(infoSource),
		DepositFFVaultStakeStraw,
		WithdrawalRequestFFVaultStraw,
		// 70
		XTRDepositAction,
		XTRBuyAction,
		// Hipo, in this order: the deposit straws stop above the mint, and
		// WithdrawHipoStakeStraw upgrades the request.
		JettonMintHipoStraw,
		DepositHipoStakeStraw,
		DepositHipoStakeDeferredStraw,
		// 75
		WithdrawHipoStakeRequestStraw,
		WithdrawHipoStakeStraw,
		WithdrawHipoStakeSettledStraw,
		WithdrawHipoStakePostponedStraw,
		JettonMintHipoRollbackStraw,
		// 80
		WithdrawHipoStakeAllRequestStraw,
		// Hipo's loans and rounds, after the staking straws: a settled round folds in the
		// bills' burning but leaves their settlements to the straws above.
		LoanRequestHipoStraw,
		LoanIssueHipoStraw,
		LoanRepaymentHipoStraw,
		PoolRoundElectionHipoStraw,
		// 85
		PoolRoundBeganHipoStraw,
		PoolRoundEndedHipoStraw,
		PoolRoundSettledHipoStraw,
		hipoRoundElectionStepStraw,
		hipoRoundEndedStepStraw,
		// 90
		hipoRoundRecoverStraw,
		hipoRoundBillStraw,
		hipoRoundMintStraw,
	}
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
		currTx := bubble.Info.(BubbleTx)
		transferBody, ok := currTx.decodedBody.Value.(abi.JettonTransferMsgBody)
		if !ok {
			return false
		}
		transferAmount := big.Int(transferBody.Amount)

		for _, child := range bubble.Children {
			internalTransferTx, ok := child.Info.(BubbleTx)
			if !ok {
				continue
			}
			if internalTransferTx.decodedBody == nil {
				continue
			}
			internalTransfer, ok := internalTransferTx.decodedBody.Value.(abi.JettonInternalTransferMsgBody)
			if !ok {
				continue
			}
			internalTransferAmount := big.Int(internalTransfer.Amount)

			if transferAmount.Cmp(&internalTransferAmount) == 0 {
				continue
			}

			return true
		}
		return false
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
		CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.JettonWallet), HasOperation(abi.JettonInternalTransferMsgOp)},
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
				CheckFuncs: []bubbleCheck{IsTx, func(bubble *Bubble) bool {
					return true
				}, HasOperation(abi.JettonNotifyMsgOp)},
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
