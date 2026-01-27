package bath

import (
	"fmt"
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/abi"
)

var ToncoSwapStraw = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{IsJettonTransfer},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		tx := bubble.Info.(BubbleJettonTransfer)
		newAction.Dex = references.Tonco
		newAction.UserWallet = tx.sender.Address
		newAction.Router = tx.recipient.Address
		newAction.In.IsTon = tx.isWrappedTon
		newAction.In.Amount = big.Int(tx.amount)
		if !newAction.In.IsTon {
			newAction.In.JettonWallet = tx.senderWallet
			newAction.In.JettonMaster = tx.master
		}
		return nil
	},
	SingleChild: &Straw[BubbleJettonSwap]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.Poolv3SwapMsgOp), HasInterface(abi.ToncoPool)},
		SingleChild: &Straw[BubbleJettonSwap]{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.PayToMsgOp), HasInterface(abi.ToncoRouter), func(bubble *Bubble) bool {
				tx := bubble.Info.(BubbleTx)
				body, ok := tx.decodedBody.Value.(abi.PayToMsgBody)
				if !ok {
					return false
				}
				if body.PayTo.SumType != "PayToCode200" && body.PayTo.SumType != "PayToCode230" { // 200 - swap, 201 - burn
					return false
				}
				return true
			}},
			Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				// exit code 200 means successful swap, 230 - failed swap
				newAction.Success = tx.decodedBody.Value.(abi.PayToMsgBody).PayTo.SumType == "PayToCode200"
				return nil
			},
			SingleChild: &Straw[BubbleJettonSwap]{
				CheckFuncs: []bubbleCheck{IsJettonTransfer, func(bubble *Bubble) bool {
					tx := bubble.Info.(BubbleJettonTransfer)
					return tx.recipient != nil
				}},
				Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
					tx := bubble.Info.(BubbleJettonTransfer)
					if *tx.recipient.Addr() != newAction.UserWallet {
						return fmt.Errorf("not a user wallet")
					}
					newAction.Out.IsTon = tx.isWrappedTon
					newAction.Out.Amount = big.Int(tx.amount)
					if !newAction.Out.IsTon {
						newAction.Out.JettonWallet = tx.recipientWallet
						newAction.Out.JettonMaster = tx.master
					}
					return nil
				},
			},
		},
	},
}

var ToncoDepositLiquiditySingleStraw = Straw[BubbleLiquidityDeposit]{
	CheckFuncs: []bubbleCheck{IsJettonTransfer, JettonRecipientAccount(references.ToncoRouter)},
	Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
		jettonTx := bubble.Info.(BubbleJettonTransfer)
		newAction.Protocol = core.Protocol{
			Name:  string(references.Tonco),
			Image: &references.ToncoImage,
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
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.Poolv3FundAccountMsgOp), HasInterface(abi.ToncoPool)},
		Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
			return nil
		},
		SingleChild: &Straw[BubbleLiquidityDeposit]{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.Accountv3AddLiquidityMsgOp)},
			Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
				return nil
			},
			SingleChild: &Straw[BubbleLiquidityDeposit]{
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.Poolv3MintMsgOp), HasInterface(abi.ToncoPool)},
				Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
					tx := bubble.Info.(BubbleTx)
					newAction.Success = tx.success
					return nil
				},
				Children: []Straw[BubbleLiquidityDeposit]{
					{
						CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.PositionnftV3PositionInitMsgOp), HasInterface(abi.NftItem)},
						Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
							tx := bubble.Info.(BubbleTx)
							newAction.Success = newAction.Success && tx.success
							return nil
						},
						SingleChild: &Straw[BubbleLiquidityDeposit]{
							Optional:   true,
							CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
						},
					},
					{
						Optional:   true,
						CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)},
						Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
							tx := bubble.Info.(BubbleTx)
							newAction.Success = newAction.Success && tx.success
							return nil
						},
					},
				},
			},
		},
	},
}

var ToncoDepositLiquidityBothStraw = Straw[BubbleLiquidityDeposit]{
	CheckFuncs: []bubbleCheck{},
	Children: []Straw[BubbleLiquidityDeposit]{
		{
			CheckFuncs: []bubbleCheck{Is(BubbleLiquidityDeposit{})},
			Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
				tx := bubble.Info.(BubbleLiquidityDeposit)
				newAction.From = tx.From
				newAction.Protocol = tx.Protocol
				newAction.Tokens = append(newAction.Tokens, tx.Tokens...)
				newAction.Success = tx.Success
				return nil
			},
		},
		{
			CheckFuncs: []bubbleCheck{IsJettonTransfer, JettonRecipientAccount(references.ToncoRouter)},
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
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.Poolv3FundAccountMsgOp), HasInterface(abi.ToncoPool)},
				Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
					return nil
				},
				SingleChild: &Straw[BubbleLiquidityDeposit]{
					CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.Accountv3AddLiquidityMsgOp)},
					Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
						return nil
					},
				},
			},
		},
	},
}

var ToncoDepositLiquidityWithRefundStraw = Straw[BubbleLiquidityDeposit]{
	CheckFuncs: []bubbleCheck{Is(BubbleLiquidityDeposit{})},
	Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
		tx := bubble.Info.(BubbleLiquidityDeposit)
		newAction.From = tx.From
		newAction.Tokens = tx.Tokens
		newAction.Protocol = tx.Protocol
		newAction.Success = tx.Success
		return nil
	},
	SingleChild: &Straw[BubbleLiquidityDeposit]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.PayToMsgOp), HasInterface(abi.ToncoRouter)},
		Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.Success = newAction.Success && tx.success
			return nil
		},
		Children: []Straw[BubbleLiquidityDeposit]{
			{
				CheckFuncs: []bubbleCheck{IsJettonTransfer},
				Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
					tx := bubble.Info.(BubbleJettonTransfer)
					amount := big.Int(tx.amount)
					for i, token := range newAction.Tokens {
						if (token.Price.Currency.Type == core.CurrencyTON && !tx.isWrappedTon) ||
							(token.Price.Currency.Type != core.CurrencyTON && *token.Price.Currency.Jetton != tx.master) {
							continue
						}

						newAction.Tokens[i].Price.Amount = *new(big.Int).Sub(&newAction.Tokens[i].Price.Amount, &amount)
					}
					newAction.Success = newAction.Success && tx.success
					return nil
				},
			},
			{
				CheckFuncs: []bubbleCheck{IsJettonTransfer},
				Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
					tx := bubble.Info.(BubbleJettonTransfer)
					amount := big.Int(tx.amount)
					for i, token := range newAction.Tokens {
						if (token.Price.Currency.Type == core.CurrencyTON && !tx.isWrappedTon) ||
							(token.Price.Currency.Type != core.CurrencyTON && *token.Price.Currency.Jetton != tx.master) {
							continue
						}

						newAction.Tokens[i].Price.Amount = *new(big.Int).Sub(&newAction.Tokens[i].Price.Amount, &amount)
					}
					newAction.Success = newAction.Success && tx.success
					return nil
				},
			},
		},
	},
}
