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
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.JettonWallet), HasOperation(abi.JettonTransferMsgOp)},
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
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.JettonWallet), HasOperation(abi.PtonTonTransferMsgOp)},
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
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.PtonTonTransferMsgOp)},
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

var StonfiSwapV2Straw = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{func(bubble *Bubble) bool {
		jettonTx, ok := bubble.Info.(BubbleJettonTransfer)
		if !ok {
			return false
		}
		if jettonTx.sender == nil {
			return false
		}
		if jettonTx.payload.SumType != abi.StonfiSwapV2JettonOp {
			return false
		}
		swap, ok := jettonTx.payload.Value.(abi.StonfiSwapV2JettonPayload)
		if !ok {
			return false
		}
		to, err := ton.AccountIDFromTlb(swap.CrossSwapBody.Receiver)
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
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.StonfiSwapV2MsgOp), HasInterface(abi.StonfiPoolV2), func(bubble *Bubble) bool {
			tx, ok := bubble.Info.(BubbleTx)
			if !ok {
				return false
			}
			if tx.additionalInfo.STONfiPool == nil {
				return false
			}
			return true
		}},
		Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			a, b := tx.additionalInfo.STONfiPool.Token0, tx.additionalInfo.STONfiPool.Token1
			body := tx.decodedBody.Value.(abi.StonfiSwapV2MsgBody)
			if body.QueryId > 0 && a.IsZero() && b.IsZero() {
				return nil
			}
			s, err := tongo.AccountIDFromTlb(body.DexPayload.TokenWallet1)
			if err != nil {
				return err
			}
			if s != nil && *s == a {
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
		Children: []Straw[BubbleJettonSwap]{
			{
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.StonfiPayToV2MsgOp)},
				Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
					tx := bubble.Info.(BubbleTx)
					newAction.Router = tx.account.Address
					return nil
				},
				SingleChild: &Straw[BubbleJettonSwap]{
					CheckFuncs: []bubbleCheck{Is(BubbleJettonTransfer{})},
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
			{
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.StonfiPayVaultV2MsgOp)},
				Optional:   true,
				SingleChild: &Straw[BubbleJettonSwap]{
					CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.StonfiDepositRefFeeV2MsgOp)},
					SingleChild: &Straw[BubbleJettonSwap]{
						CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)},
						Optional:   true,
					},
				},
			},
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
