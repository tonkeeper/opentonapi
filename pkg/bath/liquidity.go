package bath

import (
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

type BubbleLiquidityDeposit struct {
	Protocol core.Protocol
	From     tongo.AccountID
	Tokens   []core.VaultDepositInfo
	Success  bool
}

func (b BubbleLiquidityDeposit) ToAction() *Action {
	return &Action{
		LiquidityDepositAction: &LiquidityDepositAction{
			Protocol: b.Protocol,
			From:     b.From,
			Tokens:   b.Tokens,
		},
		Type:    LiquidityDeposit,
		Success: b.Success,
	}
}

var BidaskLiquidityDepositBothNativeStraw = Straw[BubbleLiquidityDeposit]{
	CheckFuncs: []bubbleCheck{IsJettonTransfer, JettonTransferOperation(abi.BidaskProvideBothJettonOp)},
	Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
		jettonTx := bubble.Info.(BubbleJettonTransfer)
		newAction.Protocol = core.Protocol{
			Name:  string(references.Bidask),
			Image: &references.BidaskImage,
		}
		newAction.From = jettonTx.sender.Address
		payload := jettonTx.payload.Value.(abi.BidaskProvideBothJettonPayload)
		tonAmount := new(big.Int)
		tonAmount.SetUint64(uint64(payload.TonAmount))
		depositTon := core.VaultDepositInfo{
			Price: core.Price{
				Currency: core.Currency{
					Type: core.CurrencyTON,
				},
				Amount: *tonAmount,
			},
			Vault: jettonTx.recipient.Address,
		}
		newAction.Tokens = append(newAction.Tokens, depositTon)
		depositJetton := core.VaultDepositInfo{
			Price: core.Price{
				Currency: core.Currency{
					Type:   core.CurrencyJetton,
					Jetton: &jettonTx.master,
				},
				Amount: big.Int(jettonTx.amount),
			},
			Vault: jettonTx.recipientWallet,
		}
		newAction.Tokens = append(newAction.Tokens, depositJetton)
		return nil
	},
	SingleChild: &Straw[BubbleLiquidityDeposit]{
		CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.BidaskRange)},
		Children: []Straw[BubbleLiquidityDeposit]{
			{
				CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.BidaskPool)},
				SingleChild: &Straw[BubbleLiquidityDeposit]{
					CheckFuncs: []bubbleCheck{IsJettonTransfer},
				},
				Optional: true,
			},
			{
				CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.BidaskLpMultitoken)},
				Children: []Straw[BubbleLiquidityDeposit]{
					{
						CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.Wallet)},
						Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
							tx := bubble.Info.(BubbleTx)
							newAction.Success = tx.success
							return nil
						},
					},
					{
						CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
					},
				},
			},
		},
	},
}

var BidaskLiquidityDepositBothJettonStraw = Straw[BubbleLiquidityDeposit]{
	CheckFuncs: []bubbleCheck{},
	Children: []Straw[BubbleLiquidityDeposit]{
		{
			CheckFuncs: []bubbleCheck{IsJettonTransfer},
			Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
				jettonTx := bubble.Info.(BubbleJettonTransfer)
				newAction.Protocol = core.Protocol{
					Name:  string(references.Bidask),
					Image: &references.BidaskImage,
				}
				newAction.From = jettonTx.sender.Address
				depositJetton := core.VaultDepositInfo{
					Price: core.Price{
						Currency: core.Currency{
							Type:   core.CurrencyJetton,
							Jetton: &jettonTx.master,
						},
						Amount: big.Int(jettonTx.amount),
					},
					Vault: jettonTx.recipientWallet,
				}
				newAction.Tokens = append(newAction.Tokens, depositJetton)
				return nil
			},
			SingleChild: &Straw[BubbleLiquidityDeposit]{
				CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.BidaskInternalLiquidityVault)},
				SingleChild: &Straw[BubbleLiquidityDeposit]{
					CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
					Optional:   true,
				},
			},
		},
		{
			CheckFuncs: []bubbleCheck{IsJettonTransfer},
			Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
				jettonTx := bubble.Info.(BubbleJettonTransfer)
				newAction.Protocol = core.Protocol{
					Name:  string(references.Bidask),
					Image: &references.BidaskImage,
				}
				newAction.From = jettonTx.sender.Address
				depositJetton := core.VaultDepositInfo{
					Price: core.Price{
						Currency: core.Currency{
							Type:   core.CurrencyJetton,
							Jetton: &jettonTx.master,
						},
						Amount: big.Int(jettonTx.amount),
					},
					Vault: jettonTx.recipientWallet,
				}
				newAction.Tokens = append(newAction.Tokens, depositJetton)
				return nil
			},
			SingleChild: &Straw[BubbleLiquidityDeposit]{
				CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.BidaskInternalLiquidityVault)},
				Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
					tx := bubble.Info.(BubbleTx)
					newAction.Success = tx.success
					return nil
				},
				SingleChild: &Straw[BubbleLiquidityDeposit]{
					CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.BidaskPool)},
					SingleChild: &Straw[BubbleLiquidityDeposit]{
						CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.BidaskRange)},
						SingleChild: &Straw[BubbleLiquidityDeposit]{
							CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.BidaskLpMultitoken)},
							Children: []Straw[BubbleLiquidityDeposit]{
								{
									CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.Wallet)},
									Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
										tx := bubble.Info.(BubbleTx)
										newAction.Success = tx.success
										return nil
									},
								},
								{
									CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
								},
							},
						},
					},
				},
			},
		},
	},
}

var BidaskLiquidityDepositJettonStraw = Straw[BubbleLiquidityDeposit]{
	CheckFuncs: []bubbleCheck{IsJettonTransfer},
	Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
		jettonTx := bubble.Info.(BubbleJettonTransfer)
		newAction.Protocol = core.Protocol{
			Name:  string(references.Bidask),
			Image: &references.BidaskImage,
		}
		newAction.From = jettonTx.sender.Address
		depositJetton := core.VaultDepositInfo{
			Price: core.Price{
				Currency: core.Currency{
					Type:   core.CurrencyJetton,
					Jetton: &jettonTx.master,
				},
				Amount: big.Int(jettonTx.amount),
			},
			Vault: jettonTx.recipientWallet,
		}
		newAction.Tokens = append(newAction.Tokens, depositJetton)
		return nil
	},
	SingleChild: &Straw[BubbleLiquidityDeposit]{
		CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.BidaskRange)},
		Children: []Straw[BubbleLiquidityDeposit]{
			{
				CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.BidaskPool)},
				SingleChild: &Straw[BubbleLiquidityDeposit]{
					CheckFuncs: []bubbleCheck{IsJettonTransfer},
				},
				Optional: true,
			},
			{
				CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.BidaskLpMultitoken)},
				Children: []Straw[BubbleLiquidityDeposit]{
					{
						CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.Wallet)},
						Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
							tx := bubble.Info.(BubbleTx)
							newAction.Success = tx.success
							return nil
						},
					},
					{
						CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
					},
				},
			},
		},
	},
}
