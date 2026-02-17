package bath

import (
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
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
	usedBubbles := map[*Bubble]struct{}{}
	var out assetTransfer
	success := true
	poolBubble, sender, in, ok := s.processMultipleRouterSwaps(b, usedBubbles)
	if !ok {
		return false
	}

	poolTx, ok := poolBubble.Info.(BubbleTx)
	if !ok {
		return false
	}
	if poolTx.inputFrom == nil {
		return false
	}
	routerAddr := poolTx.inputFrom.Address

	if !IsTx(poolBubble) && HasOperation(abi.StonfiSwapV2MsgOp)(poolBubble) && HasInterface(abi.StonfiPoolV2)(poolBubble) {
		return false
	}
	poolBubbleTx := poolBubble.Info.(BubbleTx)

	swapPayoutBubble := poolBubble.Children[0] // мы уже проверили длину в processMultipleRouterSwaps
	if len(poolBubble.Children) == 2 {         // swap with referrer payout
		referrerBubble := poolBubble.Children[0]
		swapPayoutBubble = poolBubble.Children[1]

		if !IsTx(referrerBubble) || !HasOperation(abi.StonfiPayVaultV2MsgOp)(referrerBubble) || !HasInterface(abi.StonfiRouterV2)(referrerBubble) {
			return false
		}

		usedBubbles[referrerBubble] = struct{}{}
		if len(referrerBubble.Children) > 0 {
			refPayoutBubble := referrerBubble.Children[0]

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
	if len(poolBubble.Children) > 2 { // according to docs pool payout can have maximum two children
		return false
	}

	var payoutDest tongo.AccountID
	if IsTx(swapPayoutBubble) && HasOperation(abi.StonfiPayToV2MsgOp)(swapPayoutBubble) && HasInterface(abi.StonfiRouterV2)(swapPayoutBubble) {
		if len(swapPayoutBubble.Children) != 1 {
			return false
		}

		tx := swapPayoutBubble.Info.(BubbleTx)
		body := tx.decodedBody.Value.(abi.StonfiPayToV2MsgBody)
		amount0 := big.Int(body.AdditionalInfo.Amount0Out)
		var outJettonWallet *tongo.AccountID
		var err error
		if amount0.Cmp(big.NewInt(0)) != 0 {
			outJettonWallet, err = tongo.AccountIDFromTlb(body.AdditionalInfo.Token0Address)
			if err != nil {
				return false
			}
		} else {
			outJettonWallet, err = tongo.AccountIDFromTlb(body.AdditionalInfo.Token1Address)
			if err != nil {
				return false
			}
		}
		out.JettonWallet = *outJettonWallet
		if poolBubbleTx.additionalInfo != nil {
			out.JettonMaster = poolBubbleTx.additionalInfo.JettonMasters[*outJettonWallet]
		}

		usedBubbles[swapPayoutBubble] = struct{}{}
		transferBubble := swapPayoutBubble.Children[0]

		if IsJettonTransfer(transferBubble) {
			transferTx := transferBubble.Info.(BubbleJettonTransfer)
			if transferTx.recipient == nil {
				return false
			}
			if transferTx.master == in.JettonMaster {
				// jetton master for out token is the same as for in one
				// it means that swap wasn't success and funds just sent back
				success = false
			}
			payoutDest = transferTx.recipient.Address
			out.Amount = big.Int(transferTx.amount)
			out.IsTon = transferTx.isWrappedTon
			usedBubbles[transferBubble] = struct{}{}
		} else {
			return false
		}
	} else {
		return false
	}

	// проверяем что не подменен адрес получателя свапа
	if payoutDest != *sender {
		return false
	}

	//закончили все проверки и собрали данные. билдим выходной пузырь и мержим
	var newChildren []*Bubble
	for i := range b.Children {
		if _, found := usedBubbles[b.Children[i]]; !found {
			newChildren = append(newChildren, b.Children[i])
		}
	}
	for k := range usedBubbles {
		if k != b { // не мержим валью флоу для первого элемента, т.к. он уже был учтен в b
			b.ValueFlow.Merge(k.ValueFlow)
		}
		b.Accounts = append(b.Accounts, k.Accounts...)
		b.Transaction = append(b.Transaction, k.Transaction...)
		for j := range k.Children { //прикрепляем детей от удаляемых баблов напрямую к родителю
			tb := k.Children[j]
			if _, found := usedBubbles[tb]; !found {
				newChildren = append(newChildren, tb)
			}
		}
	}

	b.Children = newChildren
	b.Info = BubbleJettonSwap{
		Dex:        references.Stonfi,
		UserWallet: *sender,
		Router:     routerAddr,
		Out:        out,
		In:         *in,
		Success:    success,
	}
	return true
}

func (s UniversalStonfiStraw) processMultipleRouterSwaps(b *Bubble, usedBubbles map[*Bubble]struct{}) (*Bubble, *tongo.AccountID, *assetTransfer, bool) {
	var sender ton.AccountID
	var in assetTransfer
	if IsJettonTransfer(b) && JettonTransferOperation(abi.StonfiSwapV2JettonOp)(b) {
		usedBubbles[b] = struct{}{}
		transfer := b.Info.(BubbleJettonTransfer)
		if transfer.sender == nil {
			return nil, nil, nil, false
		}
		if transfer.recipient == nil || !transfer.recipient.Is(abi.StonfiRouterV2) {
			return nil, nil, nil, false
		}
		sender = transfer.sender.Address
		in.Amount = big.Int(transfer.amount)
		in.JettonMaster = transfer.master
		in.JettonWallet = transfer.senderWallet
		in.IsTon = transfer.isWrappedTon
	} else {
		return nil, nil, nil, false
	}
	if len(b.Children) != 1 {
		return nil, nil, nil, false
	}

	poolB := b.Children[0]
	nextB, ok := s.processSingleRouterSwaps(poolB, usedBubbles)
	if !ok {
		return nil, nil, nil, false
	}
	if nextB == nil {
		// next bubble is nil only if payout have two children: referral payout and swap payout.
		// it means that swap is finished
		return poolB, &sender, &in, true
	}

	// now we only have swap payout, so maybe this payout is going to be transferred to another router (multiple routers swap)
	// since swap straw has only one router we will save the only last used router
	latestPoolB, _, _, ok := s.processMultipleRouterSwaps(nextB, usedBubbles)
	if ok {
		return latestPoolB, &sender, &in, true
	}

	return poolB, &sender, &in, true
}

func (s UniversalStonfiStraw) processSingleRouterSwaps(b *Bubble, usedBubbles map[*Bubble]struct{}) (*Bubble, bool) {
	if IsTx(b) && HasOperation(abi.StonfiSwapV2MsgOp)(b) && HasInterface(abi.StonfiPoolV2)(b) {
		if len(b.Children) == 0 {
			return nil, false
		}
		usedBubbles[b] = struct{}{}

		if len(b.Children) > 1 { // referral + swap payouts
			return nil, true
		}
		payoutB := b.Children[0]

		if IsTx(payoutB) && HasOperation(abi.StonfiPayToV2MsgOp)(payoutB) && HasInterface(abi.StonfiRouterV2)(payoutB) {
			if len(payoutB.Children) != 1 {
				return nil, false
			}
			usedBubbles[payoutB] = struct{}{}
			nextB := payoutB.Children[0] // can be either swap msg on the same router or jetton transfer
			latestNextB, ok := s.processSingleRouterSwaps(nextB, usedBubbles)
			if ok && latestNextB != nil {
				return latestNextB, true
			}

			return nextB, true
		}
	}

	return nil, false
}

type UniversalStonfiStraw struct{}
