package bath

import (
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/abi"
)

var MooncxSwapStraw = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.MoonSwapMsgOp), HasInterface(abi.MoonPool), func(bubble *Bubble) bool {
		tx, ok := bubble.Info.(BubbleTx)
		if !ok {
			return false
		}
		swap, ok := tx.decodedBody.Value.(abi.MoonSwapMsgBody)
		if !ok {
			return false
		}
		if swap.SwapParams.NextFulfill != nil && swap.SwapParams.NextFulfill.Recipient != tx.inputFrom.Address.ToMsgAddress() {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Dex = references.Mooncx
		newAction.UserWallet = tx.inputFrom.Address
		newAction.Router = tx.account.Address
		body := tx.decodedBody.Value.(abi.MoonSwapMsgBody)
		amount := big.Int(body.Amount)
		newAction.In.Amount = amount
		newAction.In.IsTon = true
		return nil
	},
	SingleChild: &Straw[BubbleJettonSwap]{
		CheckFuncs: []bubbleCheck{IsJettonTransfer},
		Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
			tx := bubble.Info.(BubbleJettonTransfer)
			newAction.Out.JettonMaster = tx.master
			newAction.Out.JettonWallet = tx.senderWallet
			amount := big.Int(tx.amount)
			newAction.Out.Amount = amount
			newAction.Success = tx.success
			return nil
		},
	},
}

var MooncxSwapStrawReverse = Straw[BubbleJettonSwap]{
	CheckFuncs: []bubbleCheck{func(bubble *Bubble) bool {
		tx, ok := bubble.Info.(BubbleJettonTransfer)
		if !ok {
			return false
		}
		swap, ok := tx.payload.Value.(abi.MoonSwapJettonPayload)
		if !ok {
			return false
		}
		if swap.SwapParams.NextFulfill != nil && swap.SwapParams.NextFulfill.Recipient != tx.sender.Address.ToMsgAddress() {
			return false
		}
		return true
	}},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		tx := bubble.Info.(BubbleJettonTransfer)
		newAction.Dex = references.Mooncx
		newAction.UserWallet = tx.sender.Address
		newAction.Router = tx.recipient.Address
		newAction.In.JettonMaster = tx.master
		newAction.In.JettonWallet = tx.senderWallet
		amount := big.Int(tx.amount)
		newAction.In.Amount = amount
		body := tx.payload.Value.(abi.MoonSwapJettonPayload)
		newAction.Out.Amount = big.Int(body.SwapParams.MinOut)
		newAction.Out.IsTon = true
		return nil
	},
	SingleChild: &Straw[BubbleJettonSwap]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.MoonSwapSucceedMsgOp)},
		Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.Success = tx.success
			return nil
		},
	},
}

var MoocxLiquidityDepositJettonStraw = Straw[BubbleLiquidityDeposit]{
	CheckFuncs: []bubbleCheck{IsJettonTransfer, JettonTransferOperation(abi.MoonDepositLiquidityJettonOp), func(bubble *Bubble) bool {
		jettonTx := bubble.Info.(BubbleJettonTransfer)
		return jettonTx.recipient != nil
	}},
	Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
		jettonTx := bubble.Info.(BubbleJettonTransfer)
		newAction.Protocol = core.Protocol{
			Name:  string(references.Mooncx),
			Image: &references.MooncxImage,
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
			Vault: jettonTx.recipient.Address,
		}
		newAction.Tokens = append(newAction.Tokens, depositJetton)
		return nil
	},
	SingleChild: &Straw[BubbleLiquidityDeposit]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.MoonDepositRecordMsgOp)},
		Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.Success = tx.success
			return nil
		},
		Children: []Straw[BubbleLiquidityDeposit]{
			{
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.MoonProvideLiquidityMsgOp), HasInterface(abi.MoonPool)},
				Children: []Straw[BubbleLiquidityDeposit]{
					{
						CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonInternalTransferMsgOp)},
						Children: []Straw[BubbleLiquidityDeposit]{
							{
								CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)},
							},
							{
								CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
								Optional:   true,
							},
						},
					},
					{
						CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.MoonProvideLiquiditySucceedMsgOp)},
						Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
							tx := bubble.Info.(BubbleTx)
							newAction.Success = tx.success
							return nil
						},
					},
				},
				Optional: true,
			},
			{
				CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
				Optional:   true,
			},
		},
	},
}

var MoocxLiquidityDepositNativeStraw = Straw[BubbleLiquidityDeposit]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.MoonDepositLiquidityMsgOp)},
	Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Protocol = core.Protocol{
			Name:  string(references.Mooncx),
			Image: &references.MooncxImage,
		}
		newAction.From = tx.inputFrom.Address
		tonAmount := new(big.Int)
		tonAmount.SetInt64(tx.inputAmount)
		depositTon := core.VaultDepositInfo{
			Price: core.Price{
				Currency: core.Currency{
					Type: core.CurrencyTON,
				},
				Amount: *tonAmount,
			},
			Vault: tx.account.Address,
		}
		newAction.Tokens = append(newAction.Tokens, depositTon)
		return nil
	},
	SingleChild: &Straw[BubbleLiquidityDeposit]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.MoonDepositRecordMsgOp)},
		Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.Success = tx.success
			return nil
		},
		Children: []Straw[BubbleLiquidityDeposit]{
			{
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.MoonProvideLiquidityMsgOp), HasInterface(abi.MoonPool)},
				Children: []Straw[BubbleLiquidityDeposit]{
					{
						CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.JettonInternalTransferMsgOp)},
						Children: []Straw[BubbleLiquidityDeposit]{
							{
								CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ExcessMsgOp)},
							},
							{
								CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
								Optional:   true,
							},
						},
					},
					{
						CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.MoonProvideLiquiditySucceedMsgOp)},
						Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
							tx := bubble.Info.(BubbleTx)
							newAction.Success = tx.success
							return nil
						},
					},
				},
				Optional: true,
			},
			{
				CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
				Optional:   true,
			},
		},
	},
}

var MoocxLiquidityDepositBothStraw = Straw[BubbleLiquidityDeposit]{
	CheckFuncs: []bubbleCheck{},
	Children: []Straw[BubbleLiquidityDeposit]{
		{
			CheckFuncs: []bubbleCheck{func(bubble *Bubble) bool {
				tx, ok := bubble.Info.(BubbleLiquidityDeposit)
				if !ok {
					return false
				}
				if tx.Protocol.Name != string(references.Mooncx) {
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
		},
		{
			CheckFuncs: []bubbleCheck{func(bubble *Bubble) bool {
				tx, ok := bubble.Info.(BubbleLiquidityDeposit)
				if !ok {
					return false
				}
				if tx.Protocol.Name != string(references.Mooncx) {
					return false
				}
				return true
			}},
			Builder: func(newAction *BubbleLiquidityDeposit, bubble *Bubble) error {
				tx := bubble.Info.(BubbleLiquidityDeposit)
				newAction.Tokens = append(newAction.Tokens, tx.Tokens...)
				newAction.Success = tx.Success
				return nil
			},
		},
	},
}
