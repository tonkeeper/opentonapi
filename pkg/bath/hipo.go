package bath

import (
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

// Hipo (https://hipo.finance) liquid staking: GRAM in, hGRAM out. The message flows and the
// rules these straws follow (show both sides, read the owner from the message, anchor on an
// address only Hipo can send from) are documented at
// https://github.com/HipoFinance/contract/blob/main/docs/integration.md#explorer-actions

// hipoOwner reads the owner Hipo writes into every proxied message.
func hipoOwner(addr tlb.MsgAddress) (tongo.AccountID, bool) {
	owner, err := ton.AccountIDFromTlb(addr)
	if err != nil || owner == nil {
		return tongo.AccountID{}, false
	}
	return *owner, true
}

// hipoCredit lists the owner among the bubble's accounts, so that ForAccount keeps the action
// even when the owner has no transaction in the bubble.
func hipoCredit(bubble *Bubble, owner tongo.AccountID) {
	bubble.Accounts = append(bubble.Accounts, owner)
}

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

// JettonMintHipoStraw covers both an instant stake and a deferred one settling at round end:
//
//	parent -> tokens_minted -> hGRAM wallet -> transfer_notification -> owner
var JettonMintHipoStraw = Straw[BubbleJettonMint]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceTokensMintedMsgOp), hipoFromParent},
	Builder: func(newAction *BubbleJettonMint, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.master = references.HipoParent
		newAction.recipientWallet = tx.account.Address
		newAction.success = tx.success
		body, ok := tx.decodedBody.Value.(abi.HipoFinanceTokensMintedMsgBody)
		if !ok {
			return nil
		}
		newAction.amount = body.Tokens
		if owner, ok := hipoOwner(body.Owner); ok {
			newAction.recipient = Account{Address: owner}
			hipoCredit(bubble, owner)
		}
		return nil
	},
	ValueFlowUpdater: func(newAction *BubbleJettonMint, flow *ValueFlow) {
		if newAction.success && !newAction.recipient.Address.IsZero() {
			flow.AddJettons(newAction.recipient.Address, newAction.master, big.Int(newAction.amount))
		}
	},
	Children: []Straw[BubbleJettonMint]{
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
}

// JettonBurnHipoUnstakeAllStraw names the owner of an unstake-all burn, which the wallet sends
// to itself, from the proxy_reserve_tokens the parent accepted:
//
//	hGRAM wallet -> burn -> hGRAM wallet -> proxy_reserve_tokens -> parent
var JettonBurnHipoUnstakeAllStraw = Straw[BubbleJettonBurn]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonBurnMsgOp), hipoSelfBurn},
	Builder: func(newAction *BubbleJettonBurn, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.master = references.HipoParent
		newAction.senderWallet = tx.account.Address
		newAction.sender = tx.account
		newAction.success = tx.success
		if body, ok := tx.decodedBody.Value.(abi.JettonBurnMsgBody); ok {
			newAction.amount = body.Amount
		}
		if owner, ok := hipoReserveOwner(bubble); ok {
			newAction.sender = Account{Address: owner}
			hipoCredit(bubble, owner)
		}
		return nil
	},
}

func hipoSelfBurn(bubble *Bubble) bool {
	tx := bubble.Info.(BubbleTx)
	if tx.inputFrom == nil || tx.inputFrom.Address != tx.account.Address {
		return false
	}
	_, ok := hipoReserveOwner(bubble)
	return ok
}

// hipoReserveOwner requires the parent's transaction to succeed: it throws unless the sender
// is the named owner's own wallet.
func hipoReserveOwner(bubble *Bubble) (tongo.AccountID, bool) {
	for _, child := range bubble.Children {
		tx, ok := child.Info.(BubbleTx)
		if !ok || !tx.success || tx.account.Address != references.HipoParent ||
			!tx.operation(abi.HipoFinanceProxyReserveTokensMsgOp) {
			continue
		}
		if body, ok := tx.decodedBody.Value.(abi.HipoFinanceProxyReserveTokensMsgBody); ok {
			return hipoOwner(body.Owner)
		}
	}
	return tongo.AccountID{}, false
}

func hipoFromParent(bubble *Bubble) bool {
	tx := bubble.Info.(BubbleTx)
	return tx.inputFrom != nil && tx.inputFrom.Address == references.HipoParent
}

func hipoFromTreasury(bubble *Bubble) bool {
	tx := bubble.Info.(BubbleTx)
	return tx.inputFrom != nil && tx.inputFrom.Address == references.HipoTreasury
}

// hipoDepositHead matches deposit_coins and the comment-based deposit.
var hipoDepositHead = []bubbleCheck{IsTx, IsAccount(references.HipoTreasury), Or(
	HasOperation(abi.HipoFinanceDepositCoinsMsgOp),
	hipoDepositComment,
)}

func hipoDepositComment(bubble *Bubble) bool {
	return HasTextComment("d")(bubble) || HasTextComment("D")(bubble)
}

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
	if owner, ok := hipoOwner(body.Owner); ok {
		newAction.Staker = owner
	}
	// Zero means "everything after gas"; the proxied message below carries the resolved amount.
	amount := big.Int(body.Coins)
	if amount.Sign() == 0 {
		newAction.Amount = core.PriceNanoGram(tx.inputAmount)
		return nil
	}
	newAction.Amount = core.PriceNanoGram(amount.Int64())
	return nil
}

// DepositHipoStakeStraw stops at the parent and leaves tokens_minted to JettonMintHipoStraw:
//
//	deposit_coins -> treasury -> proxy_tokens_minted -> parent
var DepositHipoStakeStraw = Straw[BubbleDepositStake]{
	CheckFuncs: hipoDepositHead,
	Builder:    hipoDepositBuilder,
	SingleChild: &Straw[BubbleDepositStake]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceProxyTokensMintedMsgOp)},
		Builder: func(newAction *BubbleDepositStake, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.Success = tx.success
			body, ok := tx.decodedBody.Value.(abi.HipoFinanceProxyTokensMintedMsgBody)
			if !ok {
				return nil
			}
			coins := big.Int(body.Coins)
			newAction.Amount = core.PriceNanoGram(coins.Int64())
			if owner, ok := hipoOwner(body.Owner); ok {
				newAction.Staker = owner
				hipoCredit(bubble, owner)
			}
			return nil
		},
	},
}

// DepositHipoStakeDeferredStraw: the hGRAM arrives at round end, in a later trace.
//
//	deposit_coins -> treasury -> proxy_save_coins -> parent -> save_coins -> hGRAM wallet
//	                          -> mint_bill -> collection -> assign_bill -> bill ->
//	                             ownership_assigned -> staker
var DepositHipoStakeDeferredStraw = Straw[BubbleDepositStake]{
	CheckFuncs: hipoDepositHead,
	Builder:    hipoDepositBuilder,
	Children: []Straw[BubbleDepositStake]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceProxySaveCoinsMsgOp)},
			Builder: func(newAction *BubbleDepositStake, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				body, ok := tx.decodedBody.Value.(abi.HipoFinanceProxySaveCoinsMsgBody)
				if !ok {
					return nil
				}
				coins := big.Int(body.Coins)
				newAction.Amount = core.PriceNanoGram(coins.Int64())
				if owner, ok := hipoOwner(body.Owner); ok {
					newAction.Staker = owner
					hipoCredit(bubble, owner)
				}
				return nil
			},
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

func hipoRolledBack(bubble *Bubble) bool {
	return HasChild(IsTx, HasOperation(abi.HipoFinanceProxyRollbackUnstakeMsgOp))(bubble)
}

// WithdrawHipoStakeRequestStraw is the head both unstakes share. It starts below the burn,
// which stays a JettonBurn action, and is pinned to the parent because anyone may send
// reserve_tokens to the treasury. The instant payout is left to WithdrawHipoStakeStraw.
//
//	burn -> hGRAM wallet -> proxy_reserve_tokens -> parent -> reserve_tokens -> treasury
var WithdrawHipoStakeRequestStraw = Straw[BubbleWithdrawStakeRequest]{
	CheckFuncs: []bubbleCheck{IsTx, IsAccount(references.HipoParent), HasOperation(abi.HipoFinanceProxyReserveTokensMsgOp)},
	Builder: func(newAction *BubbleWithdrawStakeRequest, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Implementation = core.StakingImplementationHipo
		newAction.attachedAmount = tx.inputAmount
		body, ok := tx.decodedBody.Value.(abi.HipoFinanceProxyReserveTokensMsgBody)
		if !ok {
			return nil
		}
		master := references.HipoParent
		newAction.Amount = &core.Price{
			Currency: core.Currency{Type: core.CurrencyJetton, Jetton: &master},
			Amount:   big.Int(body.Tokens),
		}
		if owner, ok := hipoOwner(body.Owner); ok {
			newAction.Staker = owner
			hipoCredit(bubble, owner)
		}
		return nil
	},
	SingleChild: &Straw[BubbleWithdrawStakeRequest]{
		CheckFuncs: []bubbleCheck{IsTx, IsAccount(references.HipoTreasury), HasOperation(abi.HipoFinanceReserveTokensMsgOp)},
		Builder: func(newAction *BubbleWithdrawStakeRequest, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.Pool = tx.account.Address
			newAction.Success = tx.success && !hipoRolledBack(bubble)
			return nil
		},
		ValueFlowUpdater: func(newAction *BubbleWithdrawStakeRequest, flow *ValueFlow) {
			// JettonBurnStraw books nothing without a burn_notification; a rollback is put
			// back by JettonMintHipoRollbackStraw.
			if newAction.Amount != nil && !newAction.Staker.IsZero() {
				flow.SubJettons(newAction.Staker, references.HipoParent, newAction.Amount.Amount)
			}
		},
		SingleChild: &Straw[BubbleWithdrawStakeRequest]{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceMintBillMsgOp)},
			Optional:   true,
			SingleChild: &Straw[BubbleWithdrawStakeRequest]{
				CheckFuncs: []bubbleCheck{hipoBillAssigned},
				Children:   optionalNotifyExcessAndDeploy[BubbleWithdrawStakeRequest](),
			},
		},
	},
}

// WithdrawHipoStakeStraw completes an instant unstake, net of the prepaid gas like
// WithdrawLiquidStake:
//
//	treasury -> proxy_tokens_burned -> parent -> tokens_burned -> hGRAM wallet ->
//	withdrawal_notification -> staker
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

// hipoSettledBill is a round-end bill settlement: paid out, postponed, or rolled back.
var hipoSettledBill = []bubbleCheck{IsTx, IsAccount(references.HipoTreasury), HasOperation(abi.HipoFinanceBurnTokensMsgOp)}

// WithdrawHipoStakeSettledStraw is the round-end payout of a deferred unstake:
//
//	collection -> burn_tokens -> treasury -> proxy_tokens_burned -> parent ->
//	tokens_burned -> hGRAM wallet -> withdrawal_notification -> staker
var WithdrawHipoStakeSettledStraw = Straw[BubbleWithdrawStake]{
	CheckFuncs: hipoSettledBill,
	Builder: func(newAction *BubbleWithdrawStake, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Pool = tx.account.Address
		newAction.Implementation = core.StakingImplementationHipo
		return nil
	},
	SingleChild: &Straw[BubbleWithdrawStake]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceProxyTokensBurnedMsgOp)},
		Builder: func(newAction *BubbleWithdrawStake, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			body, ok := tx.decodedBody.Value.(abi.HipoFinanceProxyTokensBurnedMsgBody)
			if !ok {
				return nil
			}
			if owner, ok := hipoOwner(body.Owner); ok {
				newAction.Staker = owner
				hipoCredit(bubble, owner)
			}
			return nil
		},
		SingleChild: &Straw[BubbleWithdrawStake]{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceTokensBurnedMsgOp)},
			SingleChild: &Straw[BubbleWithdrawStake]{
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceWithdrawalNotificationMsgOp)},
				Builder: func(newAction *BubbleWithdrawStake, bubble *Bubble) error {
					newAction.Amount = bubble.Info.(BubbleTx).inputAmount
					return nil
				},
			},
		},
	},
}

// WithdrawHipoStakePostponedStraw is a bill re-minted for a later round; the unstake stays
// pending:
//
//	collection -> burn_tokens -> treasury -> mint_bill -> next collection ->
//	assign_bill -> bill -> ownership_assigned -> staker
var WithdrawHipoStakePostponedStraw = Straw[BubbleWithdrawStakeRequest]{
	CheckFuncs: hipoSettledBill,
	Builder: func(newAction *BubbleWithdrawStakeRequest, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Pool = tx.account.Address
		newAction.Success = tx.success
		newAction.Implementation = core.StakingImplementationHipo
		body, ok := tx.decodedBody.Value.(abi.HipoFinanceBurnTokensMsgBody)
		if !ok {
			return nil
		}
		master := references.HipoParent
		newAction.Amount = &core.Price{
			Currency: core.Currency{Type: core.CurrencyJetton, Jetton: &master},
			Amount:   big.Int(body.Tokens),
		}
		if owner, ok := hipoOwner(body.Owner); ok {
			newAction.Staker = owner
			hipoCredit(bubble, owner)
		}
		return nil
	},
	SingleChild: &Straw[BubbleWithdrawStakeRequest]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceMintBillMsgOp)},
		SingleChild: &Straw[BubbleWithdrawStakeRequest]{
			CheckFuncs: []bubbleCheck{hipoBillAssigned},
			Children:   optionalNotifyExcessAndDeploy[BubbleWithdrawStakeRequest](),
		},
	},
}

// JettonMintHipoRollbackStraw is hGRAM put back after the treasury declined an unstake. It
// is pinned to the parent as destination: the treasury answers anyone's reserve_tokens with
// this message, sent back to them and naming whichever owner they chose.
//
//	treasury -> proxy_rollback_unstake -> parent -> rollback_unstake -> hGRAM wallet
var JettonMintHipoRollbackStraw = Straw[BubbleJettonMint]{
	CheckFuncs: []bubbleCheck{IsTx, IsAccount(references.HipoParent), HasOperation(abi.HipoFinanceProxyRollbackUnstakeMsgOp), hipoFromTreasury},
	Builder: func(newAction *BubbleJettonMint, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.master = references.HipoParent
		body, ok := tx.decodedBody.Value.(abi.HipoFinanceProxyRollbackUnstakeMsgBody)
		if !ok {
			return nil
		}
		// hGRAM, despite the Coins type.
		newAction.amount = body.Tokens
		if owner, ok := hipoOwner(body.Owner); ok {
			newAction.recipient = Account{Address: owner}
			hipoCredit(bubble, owner)
		}
		return nil
	},
	ValueFlowUpdater: func(newAction *BubbleJettonMint, flow *ValueFlow) {
		if newAction.success && !newAction.recipient.Address.IsZero() {
			flow.AddJettons(newAction.recipient.Address, newAction.master, big.Int(newAction.amount))
		}
	},
	SingleChild: &Straw[BubbleJettonMint]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceRollbackUnstakeMsgOp)},
		Builder: func(newAction *BubbleJettonMint, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.recipientWallet = tx.account.Address
			newAction.success = tx.success
			return nil
		},
		SingleChild: &Straw[BubbleJettonMint]{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)},
			Optional:   true,
		},
	},
}
