package bath

import (
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"math/big"
)

type Dex string

const (
	Stonfi    Dex = "stonfi"
	Megatonfi Dex = "megatonfi"
	Dedust    Dex = "dedust"
)

// BubbleJettonSwap contains information about a jetton swap operation at a dex.
type BubbleJettonSwap struct {
	Dex        Dex
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
	CheckFuncs: []bubbleCheck{Is(BubbleJettonTransfer{})},
	Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
		newAction.Dex = Stonfi
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
				CheckFuncs: []bubbleCheck{Is(BubbleJettonTransfer{}), Or(JettonTransferOpCode(0xc64370e5), JettonTransferOpCode(0x5ffe1295))}, //todo: rewrite after jetton operation decoding and found what doest these codes mean
				Builder: func(newAction *BubbleJettonSwap, bubble *Bubble) error {
					jettonTx := bubble.Info.(BubbleJettonTransfer)
					if jettonTx.senderWallet != newAction.Out.JettonWallet {
						return nil
					}
					newAction.Out.JettonMaster = jettonTx.master
					newAction.Out.Amount = big.Int(jettonTx.amount)
					newAction.Out.IsTon = jettonTx.isWrappedTon
					newAction.Success = true
					return nil
				},
			},
		},
	},
}

// https://dev.tonviewer.com/transaction/e19381edd8f05922eeba3c31f4b8b4b737478b4ca7b37130bdbbfd7bfa773227
// todo: add liquidity (mint lp tokens)
var StonfiMintStraw = Straw[BubbleJettonMint]{}
