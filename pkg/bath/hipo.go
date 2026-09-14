package bath

import (
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/ton"
)

// Hipo (https://hipo.finance) is a liquid-staking protocol for the native coin: GRAM in,
// hGRAM jettons out, the same shape as Tonstakers. Each flow has an instant and a deferred
// variant; the deferred one mints an SBT ("bill") that is redeemed when the round ends.
// The message flows these straws follow are documented at
// https://github.com/HipoFinance/contract/blob/main/docs/integration.md#explorer-actions

// hipoBillAssigned matches a bill transaction either as a plain assign_bill or already
// merged into a BubbleNftTransfer by NftTransferNotifyStraw, which usually claims it first.
func hipoBillAssigned(bubble *Bubble) bool {
	if _, ok := bubble.Info.(BubbleNftTransfer); ok {
		return true
	}
	tx, ok := bubble.Info.(BubbleTx)
	return ok && tx.operation(abi.HipoFinanceAssignBillMsgOp)
}

func optionalNotifyExcessAndDeploy[T actioner]() []Straw[T] {
	return []Straw[T]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.NftOwnershipAssignedMsgOp)},
			Optional:   true,
		},
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)},
			Optional:   true,
		},
		{
			CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
			Optional:   true,
		},
	}
}

// hipoDepositBuilder fills in everything that can be read from the deposit_coins message
// itself. Both deposit straws share it; the instant one then overwrites Amount with the
// exact figure from tokens_minted.
func hipoDepositBuilder(newAction *BubbleDepositStake, bubble *Bubble) error {
	tx := bubble.Info.(BubbleTx)
	newAction.Pool = tx.account.Address
	newAction.Success = tx.success
	newAction.Implementation = core.StakingImplementationHipo
	if tx.inputFrom != nil {
		newAction.Staker = tx.inputFrom.Address
	}
	body, ok := tx.decodedBody.Value.(abi.HipoFinanceDepositCoinsMsgBody)
	if !ok {
		newAction.Amount = core.PriceNanoGram(tx.inputAmount)
		return nil
	}
	// owner is addr_none for ordinary wallets, in which case the treasury credits the
	// sender. Protocols depositing on behalf of a user set it explicitly.
	if owner, err := ton.AccountIDFromTlb(body.Owner); err == nil && owner != nil {
		newAction.Staker = *owner
	}
	// coins may be zero, meaning "everything left after the gas prepayment", so fall back
	// to the attached value.
	amount := big.Int(body.Coins)
	if amount.Sign() == 0 {
		newAction.Amount = core.PriceNanoGram(tx.inputAmount)
		return nil
	}
	newAction.Amount = core.PriceNanoGram(amount.Int64())
	return nil
}

// DepositHipoStakeStraw recognizes an instant stake:
//
//	deposit_coins -> treasury -> proxy_tokens_minted -> parent -> tokens_minted ->
//	hGRAM wallet -> transfer_notification -> staker
var DepositHipoStakeStraw = Straw[BubbleDepositStake]{
	CheckFuncs: []bubbleCheck{IsTx, IsAccount(references.HipoTreasury), HasOperation(abi.HipoFinanceDepositCoinsMsgOp)},
	Builder:    hipoDepositBuilder,
	SingleChild: &Straw[BubbleDepositStake]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceProxyTokensMintedMsgOp)},
		SingleChild: &Straw[BubbleDepositStake]{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceTokensMintedMsgOp)},
			Builder: func(newAction *BubbleDepositStake, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				newAction.Success = tx.success
				body, ok := tx.decodedBody.Value.(abi.HipoFinanceTokensMintedMsgBody)
				if !ok {
					return nil
				}
				// tokens_minted carries the GRAM the treasury actually accepted.
				coins := big.Int(body.Coins)
				newAction.Amount = core.PriceNanoGram(coins.Int64())
				if owner, err := ton.AccountIDFromTlb(body.Owner); err == nil && owner != nil {
					newAction.Staker = *owner
				}
				return nil
			},
			Children: []Straw[BubbleDepositStake]{
				{
					CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonNotifyMsgOp)},
					Optional:   true,
				},
				{
					CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)},
					Optional:   true,
				},
				{
					CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
					Optional:   true,
				},
			},
		},
	},
}

// DepositHipoStakeDeferredStraw recognizes a stake made while a round is running. It is
// still a DepositStake: the GRAM has left the staker, only the hGRAM arrives at round end.
//
//	deposit_coins -> treasury -> proxy_save_coins -> parent -> save_coins -> hGRAM wallet
//	                          -> mint_bill -> collection -> assign_bill -> bill ->
//	                             ownership_assigned -> staker
var DepositHipoStakeDeferredStraw = Straw[BubbleDepositStake]{
	CheckFuncs: []bubbleCheck{IsTx, IsAccount(references.HipoTreasury), HasOperation(abi.HipoFinanceDepositCoinsMsgOp)},
	Builder:    hipoDepositBuilder,
	Children: []Straw[BubbleDepositStake]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceProxySaveCoinsMsgOp)},
			SingleChild: &Straw[BubbleDepositStake]{
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceSaveCoinsMsgOp)},
				Builder: func(newAction *BubbleDepositStake, bubble *Bubble) error {
					newAction.Success = bubble.Info.(BubbleTx).success
					return nil
				},
				SingleChild: &Straw[BubbleDepositStake]{
					CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
					Optional:   true,
				},
			},
		},
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceMintBillMsgOp)},
			SingleChild: &Straw[BubbleDepositStake]{
				CheckFuncs: []bubbleCheck{hipoBillAssigned},
				Children:   optionalNotifyExcessAndDeploy[BubbleDepositStake](),
			},
		},
	},
}

// WithdrawHipoStakeRequestStraw recognizes the head both unstake variants share, starting
// from the hGRAM burn that JettonBurnStraw has already merged:
//
//	burn -> hGRAM wallet -> proxy_reserve_tokens -> parent -> reserve_tokens -> treasury
//
// The deferred variant's mint_bill is matched here and the action stays a request. The
// instant variant's proxy_tokens_burned is left for WithdrawHipoStakeStraw. A treasury that
// answers with proxy_rollback_unstake refused the unstake, so it is not matched at all.
var WithdrawHipoStakeRequestStraw = Straw[BubbleWithdrawStakeRequest]{
	CheckFuncs: []bubbleCheck{Is(BubbleJettonBurn{}), func(bubble *Bubble) bool {
		return bubble.Info.(BubbleJettonBurn).master == references.HipoParent
	}},
	Builder: func(newAction *BubbleWithdrawStakeRequest, bubble *Bubble) error {
		burn := bubble.Info.(BubbleJettonBurn)
		newAction.Staker = burn.sender.Address
		newAction.Implementation = core.StakingImplementationHipo
		master := burn.master
		newAction.Amount = &core.Price{
			Currency: core.Currency{
				Type:   core.CurrencyJetton,
				Jetton: &master,
			},
			Amount: big.Int(burn.amount),
		}
		return nil
	},
	SingleChild: &Straw[BubbleWithdrawStakeRequest]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceProxyReserveTokensMsgOp)},
		Builder: func(newAction *BubbleWithdrawStakeRequest, bubble *Bubble) error {
			// Hipo refunds what is left of this gas prepayment together with the payout.
			newAction.attachedAmount = bubble.Info.(BubbleTx).inputAmount
			return nil
		},
		SingleChild: &Straw[BubbleWithdrawStakeRequest]{
			CheckFuncs: []bubbleCheck{
				IsTx,
				IsAccount(references.HipoTreasury),
				HasOperation(abi.HipoFinanceReserveTokensMsgOp),
				Not(HasChild(IsTx, HasOperation(abi.HipoFinanceProxyRollbackUnstakeMsgOp))),
			},
			Builder: func(newAction *BubbleWithdrawStakeRequest, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				newAction.Pool = tx.account.Address
				newAction.Success = tx.success
				return nil
			},
			Children: []Straw[BubbleWithdrawStakeRequest]{
				{
					CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceMintBillMsgOp)},
					Optional:   true,
					SingleChild: &Straw[BubbleWithdrawStakeRequest]{
						CheckFuncs: []bubbleCheck{hipoBillAssigned},
						Children:   optionalNotifyExcessAndDeploy[BubbleWithdrawStakeRequest](),
					},
				},
			},
		},
	},
}

// WithdrawHipoStakeStraw upgrades an instant unstake from a request to a completed
// withdrawal by consuming the payout leg WithdrawHipoStakeRequestStraw left behind:
//
//	treasury -> proxy_tokens_burned -> parent -> tokens_burned -> hGRAM wallet ->
//	withdrawal_notification -> staker, carrying the GRAM
//
// Like WithdrawLiquidStake, the amount nets out the gas the staker prepaid.
var WithdrawHipoStakeStraw = Straw[BubbleWithdrawStake]{
	CheckFuncs: []bubbleCheck{Is(BubbleWithdrawStakeRequest{}), func(bubble *Bubble) bool {
		request, ok := bubble.Info.(BubbleWithdrawStakeRequest)
		return ok && request.Implementation == core.StakingImplementationHipo
	}},
	Builder: func(newAction *BubbleWithdrawStake, bubble *Bubble) error {
		request := bubble.Info.(BubbleWithdrawStakeRequest)
		newAction.Pool = request.Pool
		newAction.Staker = request.Staker
		newAction.Implementation = request.Implementation
		newAction.Amount -= request.attachedAmount
		return nil
	},
	SingleChild: &Straw[BubbleWithdrawStake]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceProxyTokensBurnedMsgOp)},
		SingleChild: &Straw[BubbleWithdrawStake]{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceTokensBurnedMsgOp)},
			SingleChild: &Straw[BubbleWithdrawStake]{
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceWithdrawalNotificationMsgOp)},
				Builder: func(newAction *BubbleWithdrawStake, bubble *Bubble) error {
					newAction.Amount += bubble.Info.(BubbleTx).inputAmount
					return nil
				},
			},
		},
	},
}
