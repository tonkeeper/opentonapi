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

// Hipo (https://hipo.finance) is a liquid-staking protocol for the native coin: GRAM in,
// hGRAM jettons out, the same shape as Tonstakers. Each flow has an instant and a deferred
// variant; the deferred one mints an SBT ("bill") that is redeemed when the round ends.
// The message flows these straws follow are documented at
// https://github.com/HipoFinance/contract/blob/main/docs/integration.md#explorer-actions
//
// hGRAM is not a TEP-74 mint: the treasury reaches the holder's wallet through the jetton
// master with its own tokens_minted op, so JettonMintFromMasterStraw never fires for it.
// JettonMintHipoStraw takes that role, which is what keeps the minted amount visible next
// to the DepositStake action - the staking actions themselves carry one currency only.
// For the same reason the unstake straws start below the hGRAM burn instead of consuming
// it, so the burned amount survives as its own JettonBurn action.

// hipoOwner reads an owner address out of a decoded Hipo message body. Hipo writes the
// real owner into every proxied message, which is what makes it authoritative: on a
// deposit the treasury has already resolved addr_none to the sender, and on an unstake the
// wallet stores its own owner rather than the message sender, so it stays correct even
// when a wallet burns to itself through unstake_all.
func hipoOwner(addr tlb.MsgAddress) (tongo.AccountID, bool) {
	owner, err := ton.AccountIDFromTlb(addr)
	if err != nil || owner == nil {
		return tongo.AccountID{}, false
	}
	return *owner, true
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

// JettonMintHipoStraw recognizes hGRAM arriving on a holder's wallet:
//
//	parent -> tokens_minted -> hGRAM wallet -> transfer_notification -> owner
//
// It deliberately matches the message rather than the trace around it, so it covers both
// an instant stake and the round-end settlement of a deferred one, which mints through the
// very same pair of messages.
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
			// transfer_notification goes out with ignore_errors, so a wallet too poor to
			// pay the forward fee mints without one.
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

// hipoFromParent and hipoFromTreasury guard the messages that move an hGRAM balance. Both
// ops below may be sent by anyone, and both name the holder they credit, so without an
// anchor to an address only Hipo can speak from, a contract of a scammer's could have
// tonviewer report hGRAM arriving in a stranger's wallet. The wallets enforce the same
// rule on-chain, which is why a forged copy is thrown away rather than acted on.
func hipoFromParent(bubble *Bubble) bool {
	tx := bubble.Info.(BubbleTx)
	return tx.inputFrom != nil && tx.inputFrom.Address == references.HipoParent
}

// hipoFromTreasury is only ever a partial check; see JettonMintHipoRollbackStraw for why
// the treasury's signature on a message proves less than it looks.
func hipoFromTreasury(bubble *Bubble) bool {
	tx := bubble.Info.(BubbleTx)
	return tx.inputFrom != nil && tx.inputFrom.Address == references.HipoTreasury
}

// hipoDepositHead matches the two ways a stake reaches the treasury: the deposit_coins op
// the dapp sends, and a plain GRAM transfer carrying the comment "d", which the treasury
// routes to the same handler with coins = 0. Hipo documents the comment flow for wallets
// that cannot attach a custom payload, multisigs above all, so it carries real money.
var hipoDepositHead = []bubbleCheck{IsTx, IsAccount(references.HipoTreasury), Or(
	HasOperation(abi.HipoFinanceDepositCoinsMsgOp),
	hipoDepositComment,
)}

// hipoDepositComment reports whether this is the comment-based deposit. The treasury ORs
// the byte with 0x20 before comparing, so "D" deposits just as well as "d".
func hipoDepositComment(bubble *Bubble) bool {
	return HasTextComment("d")(bubble) || HasTextComment("D")(bubble)
}

// hipoDepositBuilder fills in everything that can be read from the deposit_coins message
// itself. Both deposit straws share it; each then overwrites Amount from the message the
// treasury proxies onwards, which is the only place the resolved figure appears.
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
		// The comment flow has no body to read: the owner is the sender and the amount is
		// resolved by the treasury, so both are filled in from the proxied message below.
		newAction.Amount = core.PriceNanoGram(tx.inputAmount)
		return nil
	}
	// owner is addr_none for ordinary wallets, in which case the treasury credits the
	// sender. Protocols depositing on behalf of a user set it explicitly.
	if owner, ok := hipoOwner(body.Owner); ok {
		newAction.Staker = owner
	}
	// coins may be zero, meaning "everything left after the gas prepayment", which only
	// the treasury can resolve. Both straws overwrite this from the message it proxies
	// onwards; the attached value is a placeholder for the moment in between.
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
//	deposit_coins -> treasury -> proxy_tokens_minted -> parent
//
// It stops at the parent: the tokens_minted leg below is left to JettonMintHipoStraw so
// that the hGRAM the staker received is reported as well.
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
			// proxy_tokens_minted carries the GRAM the treasury actually accepted.
			coins := big.Int(body.Coins)
			newAction.Amount = core.PriceNanoGram(coins.Int64())
			if owner, ok := hipoOwner(body.Owner); ok {
				newAction.Staker = owner
			}
			return nil
		},
	},
}

// DepositHipoStakeDeferredStraw recognizes a stake made while a round is running. It is
// still a DepositStake: the GRAM has left the staker, only the hGRAM arrives at round end,
// through the settlement JettonMintHipoStraw reports in that later trace.
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
				// As in the instant straw: the resolved amount, not what was attached.
				coins := big.Int(body.Coins)
				newAction.Amount = core.PriceNanoGram(coins.Int64())
				if owner, ok := hipoOwner(body.Owner); ok {
					newAction.Staker = owner
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

// hipoRolledBack reports whether the treasury answered a request by handing the hGRAM
// back instead of honouring it. It applies to both rollback sites: reserve_tokens refuses
// an instant unstake it cannot fund, and burn_tokens gives up when no round is left to
// postpone a bill to.
func hipoRolledBack(bubble *Bubble) bool {
	return HasChild(IsTx, HasOperation(abi.HipoFinanceProxyRollbackUnstakeMsgOp))(bubble)
}

// WithdrawHipoStakeRequestStraw recognizes the head both unstake variants share. It starts
// at proxy_reserve_tokens rather than at the hGRAM burn above it, for two reasons: the burn
// stays a JettonBurn action of its own, so the burned amount is still displayed next to the
// GRAM figure, and proxy_reserve_tokens names the owner and the token amount itself, so
// neither has to be recovered from a wallet that the burn may since have emptied.
//
// It is pinned to the jetton master rather than only to the treasury below it because
// reserve_tokens can be sent to the treasury by anyone - mainnet has such a transaction -
// and the treasury answers it with a rollback. Wrapped in a proxy_reserve_tokens of a
// scammer's own, that would otherwise book hGRAM out of whichever holder they named.
//
//	burn -> hGRAM wallet -> proxy_reserve_tokens -> parent -> reserve_tokens -> treasury
//
// The deferred variant's mint_bill is matched here and the action stays a request. The
// instant variant's proxy_tokens_burned is left for WithdrawHipoStakeStraw. A treasury
// that answers with proxy_rollback_unstake refused the unstake: the request is reported as
// failed, and the hGRAM is deliberately not booked out, because rollback_unstake puts it
// straight back on the wallet in this same trace.
var WithdrawHipoStakeRequestStraw = Straw[BubbleWithdrawStakeRequest]{
	CheckFuncs: []bubbleCheck{IsTx, IsAccount(references.HipoParent), HasOperation(abi.HipoFinanceProxyReserveTokensMsgOp)},
	Builder: func(newAction *BubbleWithdrawStakeRequest, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Implementation = core.StakingImplementationHipo
		// Hipo refunds what is left of this gas prepayment together with the payout.
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
			// JettonBurnStraw leaves the flow alone because Hipo answers a burn with
			// proxy_reserve_tokens instead of the TEP-74 burn_notification it looks for.
			// A rollback is booked too, and JettonMintHipoRollbackStraw puts it back: the
			// hGRAM really did leave the wallet for the length of one transaction.
			if newAction.Amount != nil && !newAction.Staker.IsZero() {
				flow.SubJettons(newAction.Staker, references.HipoParent, newAction.Amount.Amount)
			}
		},
		// The rollback leg is deliberately left unmatched for
		// JettonMintHipoRollbackStraw, which reports the hGRAM coming back.
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

// hipoSettledBill matches the treasury transaction that settles one unstake bill of a
// finished round. What the treasury does with it depends on how much liquid GRAM is left:
// it pays out, postpones the bill to a later round, or - handled by
// JettonMintHipoRollbackStraw - gives the hGRAM back. The two straws below tell those
// apart by the leg they require underneath, so the head only has to find the settlement.
var hipoSettledBill = []bubbleCheck{IsTx, IsAccount(references.HipoTreasury), HasOperation(abi.HipoFinanceBurnTokensMsgOp)}

// WithdrawHipoStakeSettledStraw recognizes the payout half of a deferred unstake, which
// lands in the round-end trace rather than in the one that requested it:
//
//	collection -> burn_tokens -> treasury -> proxy_tokens_burned -> parent ->
//	tokens_burned -> hGRAM wallet -> withdrawal_notification -> staker
//
// The hGRAM left the wallet back when the request was made, so only GRAM is reported here.
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
			}
			return nil
		},
		SingleChild: &Straw[BubbleWithdrawStake]{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceTokensBurnedMsgOp)},
			SingleChild: &Straw[BubbleWithdrawStake]{
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.HipoFinanceWithdrawalNotificationMsgOp)},
				Builder: func(newAction *BubbleWithdrawStake, bubble *Bubble) error {
					// Nothing was prepaid in this trace: the staker paid the gas in the
					// trace that made the request, and the bill carried it from there.
					newAction.Amount = bubble.Info.(BubbleTx).inputAmount
					return nil
				},
			},
		},
	},
}

// WithdrawHipoStakePostponedStraw recognizes a bill the treasury could not pay out: it
// mints a fresh bill against the next round and the unstake stays pending.
//
//	collection -> burn_tokens -> treasury -> mint_bill -> next collection ->
//	assign_bill -> bill -> ownership_assigned -> staker
//
// Reporting it as a request again is what keeps the unstake from vanishing between the
// round that could not pay and the one that finally does.
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

// JettonMintHipoRollbackStraw recognizes hGRAM coming back to a wallet after the treasury
// declined an unstake:
//
//	treasury -> proxy_rollback_unstake -> parent -> rollback_unstake -> hGRAM wallet
//
// It matches the message pair on its own so that it covers both places the treasury gives
// up: reserve_tokens refusing an instant unstake it cannot fund, and burn_tokens finding no
// round left to postpone a bill to, rounds after the request. Either way rollback_unstake
// does tokens += amount on the wallet, so a mint is what actually happened - and it is what
// keeps the burn above it from reading as hGRAM the holder lost.
//
// Coming from the treasury is necessary but nowhere near sufficient. reserve_tokens
// authenticates no one: anyone may send the treasury one naming any owner, and it answers
// by returning proxy_rollback_unstake TO THE SENDER with that owner copied across. So the
// treasury genuinely does send this message to addresses of a scammer's choosing, carrying
// a victim's address. What cannot be faked is the destination - only the real jetton master
// relays it onwards to a wallet, so that is what the match is pinned to.
var JettonMintHipoRollbackStraw = Straw[BubbleJettonMint]{
	CheckFuncs: []bubbleCheck{IsTx, IsAccount(references.HipoParent), HasOperation(abi.HipoFinanceProxyRollbackUnstakeMsgOp), hipoFromTreasury},
	Builder: func(newAction *BubbleJettonMint, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.master = references.HipoParent
		body, ok := tx.decodedBody.Value.(abi.HipoFinanceProxyRollbackUnstakeMsgBody)
		if !ok {
			return nil
		}
		// Tokens is hGRAM despite the Coins type: the treasury stores the token amount it
		// is handing back, not a GRAM value.
		newAction.amount = body.Tokens
		if owner, ok := hipoOwner(body.Owner); ok {
			newAction.recipient = Account{Address: owner}
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
