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

// BubbleJettonSwap contains information about a jetton swap operation at a dex.
type BubbleJettonSwap struct {
	Dex        references.Dex
	UserWallet tongo.AccountID
	Router     tongo.AccountID
	Out        assetTransfer
	In         assetTransfer
	Success    bool
}

// https://docs.ston.fi/developer-section/dex/smart-contracts/v2/op-codes#transfer-exit-codes
const (
	// StonfiExitCode_SwapRefundSlippage Swap out token amount is less than the provided minimum value
	StonfiExitCode_SwapRefundSlippage uint32 = 0x39603190
	StonfiExitCode_SwapOk             uint32 = 0xc64370e5
)

func (b BubbleJettonSwap) ToAction() *Action {
	return &Action{
		JettonSwap: &JettonSwapAction{
			Dex:        b.Dex,
			UserWallet: b.UserWallet,
			Router:     b.Router,
			In:         b.In,
			Out:        b.Out,
		},
		Type:    JettonSwap,
		Success: b.Success,
	}
}

var StonfiSwapStraw = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{func(bubble *Bubble) bool {
		jettonTx, ok := bubble.Info.(BubbleJettonTransfer)
		if !ok {
			return false
		}
		if jettonTx.sender == nil {
			return false
		}
		if jettonTx.payload.SumType != abi.StonfiSwapJettonOp {
			return false
		}
		swap, ok := jettonTx.payload.Value.(abi.StonfiSwapJettonPayload)
		if !ok {
			return false
		}
		to, err := ton.AccountIDFromTlb(swap.ToAddress)
		if err != nil || to == nil {
			return false
		}
		if jettonTx.sender.Address != *to { //protection against invalid swaps
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		newAction.Dex = references.Stonfi
		jettonTx := bubble.Info.(BubbleJettonTransfer)
		newAction.UserWallet = jettonTx.sender.Address
		newAction.In.Amount = big.Int(jettonTx.amount)
		newAction.In.IsTon = jettonTx.isWrappedTon
		newAction.In.JettonMaster = jettonTx.master
		return nil
	},
	SingleChild: &Straw[BubbleJettonSwap]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.StonfiSwapMsgOp), HasInterface(abi.StonfiPool)},
		Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			a, b := tx.additionalInfo.STONfiPool.Token0, tx.additionalInfo.STONfiPool.Token1
			body := tx.decodedBody.Value.(abi.StonfiSwapMsgBody)
			newAction.Out.Amount = big.Int(body.MinOut)
			s, err := tongo.AccountIDFromTlb(body.SenderAddress)
			if err != nil {
				return err
			}
			if s != nil && *s == b {
				a, b = b, a
			}
			newAction.In.JettonWallet = a
			newAction.Out.JettonWallet = b
			if tx.additionalInfo != nil {
				newAction.In.JettonMaster, _ = tx.additionalInfo.JettonMaster(a)
				newAction.Out.JettonMaster, _ = tx.additionalInfo.JettonMaster(b)
			}
			return nil
		},
		SingleChild: &Straw[BubbleJettonSwap]{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.StonfiPaymentRequestMsgOp)},
			Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				newAction.Router = tx.account.Address
				return nil
			},
			SingleChild: &Straw[BubbleJettonSwap]{
				CheckFuncs: []bubbleCheck{Is(BubbleJettonTransfer{}), Or(JettonTransferOperation(abi.StonfiSwapOkJettonOp), JettonTransferOpCode(0x5ffe1295))}, //todo: rewrite after jetton operation decoding and found what doest these codes mean
				Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
					jettonTx := bubble.Info.(BubbleJettonTransfer)
					if jettonTx.senderWallet != newAction.Out.JettonWallet {
						// operation has failed,
						// stonfi's sent jettons back to the user
						return nil
					}
					newAction.Out.Amount = big.Int(jettonTx.amount)
					newAction.Out.IsTon = jettonTx.isWrappedTon
					newAction.Success = true
					return nil
				},
			},
		},
	},
}

var StonfiV1PTONStraw = Straw[BubbleJettonTransfer]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.JettonWallet), HasOperation(abi.JettonTransferMsgOp), func(bubble *Bubble) bool {
		tx := bubble.Info.(BubbleTx)
		body := tx.decodedBody.Value.(abi.JettonTransferMsgBody)
		amount := big.Int(body.Amount)
		if big.NewInt(tx.inputAmount).Cmp(&amount) < 1 {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonTransfer, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.master, _ = tx.additionalInfo.JettonMaster(tx.account.Address)
		newAction.senderWallet = tx.account.Address
		newAction.sender = tx.inputFrom
		body := tx.decodedBody.Value.(abi.JettonTransferMsgBody)
		newAction.amount = body.Amount
		newAction.isWrappedTon = true
		recipient, err := ton.AccountIDFromTlb(body.Destination)
		if err == nil && recipient != nil {
			newAction.recipient = &Account{Address: *recipient}
			bubble.Accounts = append(bubble.Accounts, *recipient)
		}
		return nil
	},
	SingleChild: &Straw[BubbleJettonTransfer]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonNotifyMsgOp)},
		Builder: func(newAction *BubbleJettonTransfer, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.success = true
			body := tx.decodedBody.Value.(abi.JettonNotifyMsgBody)
			newAction.amount = body.Amount
			newAction.payload = body.ForwardPayload.Value
			newAction.recipient = &tx.account
			return nil
		},
	},
}

var StonfiV2PTONStraw = Straw[BubbleJettonTransfer]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.JettonWallet), HasOperation(abi.PtonTonTransferMsgOp), func(bubble *Bubble) bool {
		tx := bubble.Info.(BubbleTx)
		body := tx.decodedBody.Value.(abi.PtonTonTransferMsgBody)
		if uint64(tx.inputAmount) <= uint64(body.TonAmount) {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonTransfer, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.master, _ = tx.additionalInfo.JettonMaster(tx.account.Address)
		newAction.senderWallet = tx.account.Address
		newAction.sender = tx.inputFrom
		body := tx.decodedBody.Value.(abi.PtonTonTransferMsgBody)
		newAction.amount = tlb.VarUInteger16(*big.NewInt(int64(body.TonAmount)))
		newAction.isWrappedTon = true
		recipient, err := ton.AccountIDFromTlb(body.RefundAddress)
		if err == nil {
			newAction.recipient = &Account{Address: *recipient}
		}
		return nil
	},
	SingleChild: &Straw[BubbleJettonTransfer]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonNotifyMsgOp)},
		Builder: func(newAction *BubbleJettonTransfer, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.success = true
			body := tx.decodedBody.Value.(abi.JettonNotifyMsgBody)
			newAction.amount = body.Amount
			newAction.payload = body.ForwardPayload.Value
			newAction.recipient = &tx.account
			return nil
		},
	},
}

var StonfiV2PTONStrawReverse = Straw[BubbleJettonTransfer]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.JettonWallet), HasOperation(abi.JettonTransferMsgOp)},
	Builder: func(newAction *BubbleJettonTransfer, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.master, _ = tx.additionalInfo.JettonMaster(tx.account.Address)
		newAction.senderWallet = tx.account.Address
		newAction.sender = tx.inputFrom
		body := tx.decodedBody.Value.(abi.JettonTransferMsgBody)
		newAction.amount = body.Amount
		newAction.isWrappedTon = true
		newAction.payload = body.ForwardPayload.Value
		recipient, err := ton.AccountIDFromTlb(body.Destination)
		if err == nil {
			newAction.recipient = &Account{Address: *recipient}
		}
		return nil
	},
	Children: []Straw[BubbleJettonTransfer]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.PtonTonTransferMsgOp), func(bubble *Bubble) bool {
				tx := bubble.Info.(BubbleTx)
				body := tx.decodedBody.Value.(abi.PtonTonTransferMsgBody)
				if uint64(tx.inputAmount) < uint64(body.TonAmount) {
					return false
				}
				return true
			}},
			Builder: func(newAction *BubbleJettonTransfer, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				newAction.success = true
				newAction.recipient = &tx.account
				return nil
			},
		},
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)},
			Optional:   true,
		},
	},
}

var StonfiLiquidityDepositSingle = Straw[BubbleLiquidityDeposit]{
	CheckFuncs: []bubbleCheck{IsJettonTransfer, func(bubble *Bubble) bool {
		tx := bubble.Info.(BubbleJettonTransfer)
		body, ok := tx.payload.Value.(abi.StonfiProvideLpV2JettonPayload)
		if !ok {
			return false
		}
		if body.CrossProvideLpBody.ToAddress != tx.sender.Address.ToMsgAddress() { // todo support liquidity deposit with farming
			return false
		}
		_, ok = references.StonfiWhitelistVaults[tx.recipient.Address]
		if !ok {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
		jettonTx := bubble.Info.(BubbleJettonTransfer)
		newAction.Protocol = core.Protocol{
			Name:  string(references.Stonfi),
			Image: &references.StonfiImage,
		}
		newAction.From = jettonTx.sender.Address
		if jettonTx.isWrappedTon {
			newAction.Tokens = append(newAction.Tokens, core.VaultDepositInfo{
				Price: core.Price{
					Currency: core.Currency{
						Type: core.CurrencyTON,
					},
					Amount: big.Int(jettonTx.amount),
				},
				Vault: jettonTx.recipient.Address,
			})
		} else {
			newAction.Tokens = append(newAction.Tokens, core.VaultDepositInfo{
				Price: core.Price{
					Currency: core.Currency{
						Type:   core.CurrencyJetton,
						Jetton: &jettonTx.master,
					},
					Amount: big.Int(jettonTx.amount),
				},
				Vault: jettonTx.recipientWallet,
			})
		}
		return nil
	},
	SingleChild: &Straw[BubbleLiquidityDeposit]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.StonfiProvideLpV2MsgOp), HasInterface(abi.StonfiPoolV2)},
		SingleChild: &Straw[BubbleLiquidityDeposit]{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.StonfiAddLiquidityV2MsgOp)},
			Children: []Straw[BubbleLiquidityDeposit]{
				{
					CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.StonfiCbAddLiquidityV2MsgOp), HasInterface(abi.StonfiPoolV2)},
					Children: []Straw[BubbleLiquidityDeposit]{
						{
							Optional:   true,
							CheckFuncs: []bubbleCheck{Is(BubbleJettonMint{})},
							SingleChild: &Straw[BubbleLiquidityDeposit]{
								CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.NftItem)},
								Children: []Straw[BubbleLiquidityDeposit]{
									{
										CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)},
										Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
											tx := bubble.Info.(BubbleTx)
											newAction.Success = tx.success
											return nil
										},
									},
									{
										Optional:   true,
										CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
									},
								},
							},
						},
						{
							Optional:   true,
							CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonInternalTransferMsgOp)},
							SingleChild: &Straw[BubbleLiquidityDeposit]{
								CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)},
								Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
									tx := bubble.Info.(BubbleTx)
									newAction.Success = tx.success
									return nil
								},
							},
						},
					},
				},
				{
					Optional:   true,
					CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
				},
			},
		},
	},
}

var StonfiLiquidityDepositBoth = Straw[BubbleLiquidityDeposit]{
	CheckFuncs: []bubbleCheck{},
	Children: []Straw[BubbleLiquidityDeposit]{
		{
			CheckFuncs: []bubbleCheck{func(bubble *Bubble) bool {
				tx, ok := bubble.Info.(BubbleLiquidityDeposit)
				if !ok {
					return false
				}
				if tx.Protocol.Name != string(references.Stonfi) {
					return false
				}
				return true
			}},
			Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
				tx := bubble.Info.(BubbleLiquidityDeposit)
				newAction.Protocol = tx.Protocol
				newAction.From = tx.From
				newAction.Tokens = append(newAction.Tokens, tx.Tokens...)
				return nil
			},
			Children: []Straw[BubbleLiquidityDeposit]{},
		},
		{
			CheckFuncs: []bubbleCheck{IsJettonTransfer},
			Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
				jettonTx := bubble.Info.(BubbleJettonTransfer)
				if jettonTx.isWrappedTon {
					newAction.Tokens = append(newAction.Tokens, core.VaultDepositInfo{
						Price: core.Price{
							Currency: core.Currency{
								Type: core.CurrencyTON,
							},
							Amount: big.Int(jettonTx.amount),
						},
						Vault: jettonTx.recipient.Address,
					})
				} else {
					newAction.Tokens = append(newAction.Tokens, core.VaultDepositInfo{
						Price: core.Price{
							Currency: core.Currency{
								Type:   core.CurrencyJetton,
								Jetton: &jettonTx.master,
							},
							Amount: big.Int(jettonTx.amount),
						},
						Vault: jettonTx.recipientWallet,
					})
				}
				return nil
			},
			SingleChild: &Straw[BubbleLiquidityDeposit]{
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.StonfiProvideLpV2MsgOp), HasInterface(abi.StonfiPoolV2)},
				SingleChild: &Straw[BubbleLiquidityDeposit]{
					CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.StonfiAddLiquidityV2MsgOp)},
					Children: []Straw[BubbleLiquidityDeposit]{
						{
							CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)},
							Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
								tx := bubble.Info.(BubbleTx)
								newAction.Success = tx.success
								return nil
							},
						},
						{
							Optional:   true,
							CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
						},
					},
				},
			},
		},
	},
}

func (s UniversalStonfiStraw) Merge(b *Bubble) bool {
	swapHops := s.processMultipleRouterSwaps(b)
	if len(swapHops) == 0 {
		return false
	}

	firstHop := swapHops[len(swapHops)-1]
	finalHop := swapHops[0]
	finalTransfer, ok := finalHop.srSwaps[0].transfer.Info.(BubbleJettonTransfer)
	// проверяем что не подменен адрес получателя свапа
	if !ok || finalTransfer.recipient == nil || finalTransfer.recipient.Address != firstHop.sender {
		return false
	}

	swapInfo := BubbleJettonSwap{
		Dex:        references.Stonfi,
		UserWallet: firstHop.sender,
		In:         firstHop.in,
		Router:     firstHop.router,
		Out:        firstHop.out,
		Success:    firstHop.success,
	}

	b.Children = []*Bubble{}
	mergeValueFlowAndTXs(b, firstHop.usedBubbles)

	hasFailed := !swapInfo.Success
	for i := len(swapHops) - 2; i >= 0; i-- {
		nextHop := swapHops[i]
		if !hasFailed && nextHop.success {
			// merge consequent swaps
			swapInfo.Router = nextHop.router
			swapInfo.Out = nextHop.out
			swapInfo.Success = true
			mergeValueFlowAndTXs(b, nextHop.usedBubbles)
		} else {
			hasFailed = true
			failedSwapB := &Bubble{
				Info: BubbleJettonSwap{
					Dex:        references.Stonfi,
					UserWallet: firstHop.sender,
					In:         nextHop.in,
					Router:     nextHop.router,
					Out:        nextHop.out,
					Success:    false,
				},
				ValueFlow: newValueFlow(),
				Children:  nextHop.nextBubble.Children,
			}
			mergeValueFlowAndTXs(failedSwapB, nextHop.usedBubbles)
			b.Children = append(b.Children, failedSwapB)
		}
	}

	if hasFailed && len(swapHops) > 1 {
		// swap failed at a later hop, so the output jetton ends up back at the user.
		swapInfo.Out.JettonWallet = firstHop.sender
	}
	b.Info = swapInfo
	return true
}

func mergeValueFlowAndTXs(b *Bubble, usedBubbles map[*Bubble]struct{}) {
	for k := range usedBubbles {
		if k != b { // не мержим валью флоу для первого элемента, т.к. он уже был учтен в b
			b.ValueFlow.Merge(k.ValueFlow)
		}
		b.Accounts = append(b.Accounts, k.Accounts...)
		b.Transaction = append(b.Transaction, k.Transaction...)
	}
}

// stonfiSwap represents a single swap
type stonfiSwap struct {
	swapTx     BubbleTx
	transfer   *Bubble
	payoutBody abi.StonfiPayToV2MsgBody
}

// stonfiSwap represent a swap chain involving single router
type stonfiSingleRouterSwapChain struct {
	router     tongo.AccountID
	nextBubble *Bubble
	sender     tongo.AccountID
	in         assetTransfer
	out        assetTransfer
	transfer   BubbleJettonTransfer
	// srSwaps contains intermediary swaps; the first element in the slice is the first payout made (to the next swap)
	srSwaps     []stonfiSwap
	inPayload   abi.StonfiSwapV2JettonPayload
	success     bool
	usedBubbles map[*Bubble]struct{}
	start       *Bubble
}

// stonfiSwapMultiHop contains swaps involving multiple routers; the first element is the first swap made
type stonfiSwapMultiHop []stonfiSingleRouterSwapChain

func (spo stonfiSwap) outAddress() (*tongo.AccountID, error) {
	amount0 := big.Int(spo.payoutBody.AdditionalInfo.Amount0Out)
	if amount0.Cmp(big.NewInt(0)) != 0 {
		return tongo.AccountIDFromTlb(spo.payoutBody.AdditionalInfo.Token0Address)
	}
	return tongo.AccountIDFromTlb(spo.payoutBody.AdditionalInfo.Token1Address)
}

func (s UniversalStonfiStraw) processMultipleRouterSwaps(b *Bubble) stonfiSwapMultiHop {

	if len(b.Children) != 1 || !IsJettonTransfer(b) || !JettonTransferOperation(abi.StonfiSwapV2JettonOp)(b) {
		return nil
	}
	usedBubbles := map[*Bubble]struct{}{
		b: {},
	}
	transfer, ok := b.Info.(BubbleJettonTransfer)
	if !ok || transfer.sender == nil || transfer.recipient == nil || !transfer.recipient.Is(abi.StonfiRouterV2) {
		return nil
	}
	jettonPayload, ok := transfer.payload.Value.(abi.StonfiSwapV2JettonPayload)
	if !ok {
		return nil
	}
	router := transfer.recipient.Address
	sender := transfer.sender.Address
	in := assetTransfer{
		Amount:       big.Int(transfer.amount),
		JettonMaster: transfer.master,
		JettonWallet: transfer.senderWallet,
		IsTon:        transfer.isWrappedTon,
	}

	srSwaps, nextB, ok := s.processSingleRouterSwaps(b.Children[0], usedBubbles)
	if !ok || len(srSwaps) == 0 {
		return nil
	}
	finalTransfer, ok := srSwaps[0].transfer.Info.(BubbleJettonTransfer)
	if !ok {
		return nil
	}
	outJettonWallet, err := srSwaps[0].outAddress()
	if err != nil || outJettonWallet == nil {
		return nil
	}
	success := srSwaps[0].payoutBody.ExitCode == StonfiExitCode_SwapOk
	var out assetTransfer
	if success {
		out = assetTransfer{
			Amount:       big.Int(finalTransfer.amount),
			IsTon:        finalTransfer.isWrappedTon,
			JettonWallet: *outJettonWallet,
			JettonMaster: finalTransfer.master,
		}
	} else {
		userWallet, err := tongo.AccountIDFromTlb(srSwaps[0].payoutBody.ToAddress)
		if err != nil || userWallet == nil {
			return nil
		}
		jettonPoolWallet, err := tongo.AccountIDFromTlb(jettonPayload.TokenWallet1)
		if err != nil || jettonPoolWallet == nil {
			return nil
		}
		firstSwapTx := srSwaps[len(srSwaps)-1].swapTx
		var jettonMaster tongo.AccountID
		if firstSwapTx.additionalInfo != nil {
			jettonMaster, _ = firstSwapTx.additionalInfo.JettonMaster(*jettonPoolWallet)
		}
		out = assetTransfer{
			Amount: big.Int(jettonPayload.CrossSwapBody.MinOut),
			IsTon:  false,
			// jetton wallet of user is not defined (it should have been determined by successful payout tx), so we user's normal wallet
			JettonWallet: *userWallet,
			JettonMaster: jettonMaster,
		}
	}
	thisHop := stonfiSingleRouterSwapChain{
		start:       b,
		usedBubbles: usedBubbles,
		router:      router,
		sender:      sender,
		in:          in,
		out:         out,
		nextBubble:  nextB,
		srSwaps:     srSwaps,
		inPayload:   jettonPayload,
		success:     success,
	}

	// next bubble is nil only if payout have two children: referral payout and swap payout.
	// it means that swap is finished
	if nextB != nil {
		// maybe this payout is going to be transferred to another router (multiple routers swap)
		nextHops := s.processMultipleRouterSwaps(nextB)
		if nextHops != nil {
			delete(usedBubbles, nextB)
			return append(nextHops, thisHop)
		}
	}

	return []stonfiSingleRouterSwapChain{thisHop}
}

func (s UniversalStonfiStraw) processSingleRouterSwaps(b *Bubble, usedBubbles map[*Bubble]struct{}) ([]stonfiSwap, *Bubble, bool) {
	swapTx, ok := b.Info.(BubbleTx) // same as IsTX(b)
	if !(ok && HasOperation(abi.StonfiSwapV2MsgOp)(b) && HasInterface(abi.StonfiPoolV2)(b)) {
		return nil, nil, false
	}

	if len(b.Children) == 0 || len(b.Children) > 2 { // according to docs pool payout can have maximum two children
		return nil, nil, false
	}

	payoutB := b.Children[0]
	var referrerB *Bubble

	if len(b.Children) == 2 { // swap with referrer payout
		referrerB = b.Children[0]
		payoutB = b.Children[1]

		if !IsTx(referrerB) || !HasOperation(abi.StonfiPayVaultV2MsgOp)(referrerB) || !HasInterface(abi.StonfiRouterV2)(referrerB) {
			return nil, nil, false
		}
		usedBubbles[referrerB] = struct{}{}
		if len(referrerB.Children) > 0 {
			refPayoutBubble := referrerB.Children[0]
			if IsTx(refPayoutBubble) && HasOperation(abi.StonfiDepositRefFeeV2MsgOp)(refPayoutBubble) && HasInterface(abi.StonfiVaultV2)(refPayoutBubble) {
				usedBubbles[refPayoutBubble] = struct{}{}
				if len(refPayoutBubble.Children) > 0 {
					excessBubble := refPayoutBubble.Children[0]
					if IsTx(excessBubble) && HasOperation(abi.ExcessMsgOp)(excessBubble) {
						usedBubbles[excessBubble] = struct{}{}
					}
				}
			}
		}
	}

	tx, ok := payoutB.Info.(BubbleTx)
	if !(ok && HasOperation(abi.StonfiPayToV2MsgOp)(payoutB) && HasInterface(abi.StonfiRouterV2)(payoutB)) {
		return nil, nil, false
	}
	if len(payoutB.Children) != 1 {
		return nil, nil, false
	}

	body, ok := tx.decodedBody.Value.(abi.StonfiPayToV2MsgBody)
	if !ok {
		return nil, nil, false
	}
	nextB := payoutB.Children[0] // can be either swap msg on the same router or jetton transfer
	payout := stonfiSwap{
		swapTx:     swapTx,
		payoutBody: body,
		transfer:   nextB,
	}
	usedBubbles[payoutB] = struct{}{}
	usedBubbles[nextB] = struct{}{}
	usedBubbles[b] = struct{}{}
	payoutChain, latestNextB, ok := s.processSingleRouterSwaps(nextB, usedBubbles)
	if ok && latestNextB != nil {
		return append(payoutChain, payout), latestNextB, true
	}
	return append(payoutChain, payout), nextB, true
}

type UniversalStonfiStraw struct{}
