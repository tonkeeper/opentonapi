package bath

import (
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/ton"
)

// Hipo (https://hipo.finance) is a liquid-staking protocol for the native coin. A staker
// sends GRAM to the treasury and receives hGRAM jettons; the pooled GRAM is lent to
// validators for one validation round at a time. Structurally it is the same shape as
// Tonstakers - native coin in, jetton receipt out - which is why its straws build the
// pool-shaped BubbleDepositStake / BubbleWithdrawStakeRequest / BubbleWithdrawStake
// bubbles rather than the jetton-vault-shaped BubbleDepositTokenStake.
//
// Each user-facing flow has an instant and a deferred variant. Which one runs depends on
// whether the treasury currently holds enough liquid GRAM: instant settles inside the same
// trace, deferred mints an SBT ("bill") that is redeemed when the round is finalized.
//
// The op-codes below are Hipo's, taken from contracts/schema.tlb in
// github.com/HipoFinance/contract. Only deposit_coins, proxy_tokens_minted and
// tokens_minted are declared in the tongo release this module is pinned to, so everything
// else is matched by raw opcode with HasOpcode. Once tongo ships the full
// abi/schemas/hipo_finance.xml these can be swapped for abi.HipoFinance*MsgOp names and
// the bodies of withdrawal_notification / mint_bill can be decoded for exact amounts.
const (
	hipoProxySaveCoinsMsgOpCode         = 0x47daa10f // treasury -> parent
	hipoSaveCoinsMsgOpCode              = 0x4cce0e74 // parent   -> hGRAM wallet
	hipoMintBillMsgOpCode               = 0x4b2d7871 // treasury -> round collection
	hipoAssignBillMsgOpCode             = 0x3275dfc2 // collection -> bill (SBT)
	hipoProxyReserveTokensMsgOpCode     = 0x688b0213 // hGRAM wallet -> parent
	hipoReserveTokensMsgOpCode          = 0x386a358b // parent   -> treasury
	hipoProxyTokensBurnedMsgOpCode      = 0x4476fde0 // treasury -> parent
	hipoTokensBurnedMsgOpCode           = 0x5b512e25 // parent   -> hGRAM wallet
	hipoWithdrawalNotificationMsgOpCode = 0xf0fa223b // hGRAM wallet -> staker, carries the GRAM
	hipoProxyRollbackUnstakeMsgOpCode   = 0x32b67194 // treasury -> parent, unstake refused
)

// hipoUnstakeNotRolledBack rejects the treasury's reserve_tokens transaction when it
// answered with proxy_rollback_unstake instead of releasing coins or minting a bill.
//
// The treasury rolls an unstake back when the request came from a wallet whose parent is
// no longer the current one, or when there is neither enough liquid GRAM for an instant
// unstake nor an open round to defer the payout to. It does so by sending
// proxy_rollback_unstake and then `throw(0)`, which keeps the outgoing message but leaves
// the treasury's storage untouched - so the transaction reports success and the hGRAM is
// credited straight back to the staker's wallet. Without this check the trace would look
// exactly like a deferred unstake and would be reported as a withdrawal request that never
// happened.
func hipoUnstakeNotRolledBack(bubble *Bubble) bool {
	for _, child := range bubble.Children {
		tx, ok := child.Info.(BubbleTx)
		if !ok {
			continue
		}
		if tx.opCode != nil && *tx.opCode == hipoProxyRollbackUnstakeMsgOpCode {
			return false
		}
	}
	return true
}

// hipoBillAssigned matches the transaction of a bill - the SBT that records a deferred
// stake or unstake until the round is finalized - in whichever shape it has by the time
// the Hipo straws run.
//
// A bill is deployed by assign_bill and immediately notifies its owner with the standard
// TEP-62 ownership_assigned, so NftTransferNotifyStraw, which runs much earlier, usually
// merges it into a BubbleNftTransfer and the assign_bill opcode is no longer visible. It
// stays a plain transaction when that straw cannot claim it: when the indexer does not
// recognize the bill as an NFT item, or when the staker asked for no notification by
// setting ownership_assigned_amount to zero.
func hipoBillAssigned(bubble *Bubble) bool {
	if _, ok := bubble.Info.(BubbleNftTransfer); ok {
		return true
	}
	tx, ok := bubble.Info.(BubbleTx)
	return ok && tx.opCode != nil && *tx.opCode == hipoAssignBillMsgOpCode
}

// hipoBillChildren are the leftovers hanging off a bill transaction: the owner
// notification and the excess refund when the bill is still a plain transaction, and the
// deploy bubble of the bill itself, which fromTrace appends for every freshly deployed
// account. All are optional because which of them exist depends on the shape above.
func hipoBillChildren[T actioner]() []Straw[T] {
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
	// coins is what the staker asked to stake. It may be zero, meaning "everything that
	// is left after the gas prepayment", so fall back to the attached value; that
	// overstates the stake by the unused part of the ~0.009 GRAM prepayment, which the
	// treasury refunds separately.
	amount := big.Int(body.Coins)
	if amount.Sign() == 0 {
		newAction.Amount = core.PriceNanoGram(tx.inputAmount)
		return nil
	}
	newAction.Amount = core.PriceNanoGram(amount.Int64())
	return nil
}

// DepositHipoStakeStraw recognizes an instant stake, which is what happens when the
// treasury is between rounds and can mint hGRAM right away:
//
//	deposit_coins -> treasury -> proxy_tokens_minted -> parent -> tokens_minted ->
//	hGRAM wallet -> transfer_notification -> staker
//
// It must be registered before DepositHipoStakeDeferredStraw: the deferred straw would not
// match this shape, but keeping the specific case first documents the intent.
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
				// tokens_minted carries the GRAM the treasury actually accepted, so it
				// is preferred over the deposit_coins estimate.
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
					// The staker's hGRAM wallet on their very first stake.
					CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
					Optional:   true,
				},
			},
		},
	},
}

// DepositHipoStakeDeferredStraw recognizes a stake made while a round is running. The
// treasury cannot mint hGRAM yet because the exchange rate is not final, so it records the
// coins on the staker's wallet and mints an SBT that is redeemed for hGRAM when the round
// is finalized:
//
//	deposit_coins -> treasury -> proxy_save_coins -> parent -> save_coins -> hGRAM wallet
//	                          -> mint_bill -> collection -> assign_bill -> bill ->
//	                             ownership_assigned -> staker
//
// This still reports a DepositStake: the GRAM has left the staker and the stake is
// irrevocable, only the hGRAM arrives later (in the round-finalization trace, which is
// shared by every staker of that round and is not classified here).
var DepositHipoStakeDeferredStraw = Straw[BubbleDepositStake]{
	CheckFuncs: []bubbleCheck{IsTx, IsAccount(references.HipoTreasury), HasOperation(abi.HipoFinanceDepositCoinsMsgOp)},
	Builder:    hipoDepositBuilder,
	Children: []Straw[BubbleDepositStake]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOpcode(hipoProxySaveCoinsMsgOpCode)},
			SingleChild: &Straw[BubbleDepositStake]{
				CheckFuncs: []bubbleCheck{IsTx, HasOpcode(hipoSaveCoinsMsgOpCode)},
				Builder: func(newAction *BubbleDepositStake, bubble *Bubble) error {
					newAction.Success = bubble.Info.(BubbleTx).success
					return nil
				},
				SingleChild: &Straw[BubbleDepositStake]{
					// The staker's hGRAM wallet on their very first stake.
					CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
					Optional:   true,
				},
			},
		},
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOpcode(hipoMintBillMsgOpCode)},
			SingleChild: &Straw[BubbleDepositStake]{
				CheckFuncs: []bubbleCheck{hipoBillAssigned},
				Children:   hipoBillChildren[BubbleDepositStake](),
			},
		},
	},
}

// WithdrawHipoStakeRequestStraw recognizes the head that both unstake variants share. An
// unstake starts as an ordinary TEP-74 burn of hGRAM, so by the time this straw runs
// JettonBurnStraw has already merged it into a BubbleJettonBurn - the same entry point
// Tonstakers uses in PendingWithdrawRequestLiquidStraw. This straw must therefore be
// registered after JettonBurnStraw, and it claims the trace so it is reported as an
// unstake instead of a bare jetton burn.
//
//	burn -> hGRAM wallet -> proxy_reserve_tokens -> parent -> reserve_tokens -> treasury
//
// From the treasury onwards the two variants diverge. The deferred one mints a bill for
// the round-end payout and is matched here (as an optional child), leaving the action as
// WithdrawStakeRequest. The instant one continues with proxy_tokens_burned, which is left
// unmerged for WithdrawHipoStakeStraw to pick up and upgrade to a completed WithdrawStake.
var WithdrawHipoStakeRequestStraw = Straw[BubbleWithdrawStakeRequest]{
	CheckFuncs: []bubbleCheck{Is(BubbleJettonBurn{})},
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
		CheckFuncs: []bubbleCheck{IsTx, HasOpcode(hipoProxyReserveTokensMsgOpCode)},
		Builder: func(newAction *BubbleWithdrawStakeRequest, bubble *Bubble) error {
			// The gas the staker prepaid with the burn. Hipo returns whatever is left of
			// it together with the GRAM payout, so WithdrawHipoStakeStraw nets it out.
			newAction.attachedAmount = bubble.Info.(BubbleTx).inputAmount
			return nil
		},
		SingleChild: &Straw[BubbleWithdrawStakeRequest]{
			CheckFuncs: []bubbleCheck{
				IsTx,
				IsAccount(references.HipoTreasury),
				HasOpcode(hipoReserveTokensMsgOpCode),
				hipoUnstakeNotRolledBack,
			},
			Builder: func(newAction *BubbleWithdrawStakeRequest, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				newAction.Pool = tx.account.Address
				newAction.Success = tx.success
				return nil
			},
			Children: []Straw[BubbleWithdrawStakeRequest]{
				{
					// Deferred unstake only; absent in the instant flow.
					CheckFuncs: []bubbleCheck{IsTx, HasOpcode(hipoMintBillMsgOpCode)},
					Optional:   true,
					SingleChild: &Straw[BubbleWithdrawStakeRequest]{
						CheckFuncs: []bubbleCheck{hipoBillAssigned},
						Children:   hipoBillChildren[BubbleWithdrawStakeRequest](),
					},
				},
			},
		},
	},
}

// WithdrawHipoStakeStraw upgrades an instant unstake from a request to a completed
// withdrawal by consuming the payout leg that WithdrawHipoStakeRequestStraw left behind:
//
//	treasury -> proxy_tokens_burned -> parent -> tokens_burned -> hGRAM wallet ->
//	withdrawal_notification -> staker, carrying the GRAM
//
// It must be registered after WithdrawHipoStakeRequestStraw. A deferred unstake has no
// such leg and stays a WithdrawStakeRequest.
//
// The amount is the value of withdrawal_notification minus the gas the staker prepaid,
// mirroring how WithdrawLiquidStake nets out attachedAmount: the message carries the
// withdrawn coins plus the unused part of the prepayment. withdrawal_notification also
// states the exact figure in its body, which can be read once tongo decodes op 0xf0fa223b.
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
		CheckFuncs: []bubbleCheck{IsTx, HasOpcode(hipoProxyTokensBurnedMsgOpCode)},
		SingleChild: &Straw[BubbleWithdrawStake]{
			CheckFuncs: []bubbleCheck{IsTx, HasOpcode(hipoTokensBurnedMsgOpCode)},
			SingleChild: &Straw[BubbleWithdrawStake]{
				CheckFuncs: []bubbleCheck{IsTx, HasOpcode(hipoWithdrawalNotificationMsgOpCode)},
				Builder: func(newAction *BubbleWithdrawStake, bubble *Bubble) error {
					newAction.Amount += bubble.Info.(BubbleTx).inputAmount
					return nil
				},
			},
		},
	},
}
