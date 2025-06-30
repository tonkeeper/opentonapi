package bath

import (
	"github.com/tonkeeper/tongo"
	"math/big"
)

type BubbleAirdropClaim struct {
	Distributor           tongo.AccountID
	Recipient             *Account
	RecipientJettonWallet tongo.AccountID
	ClaimedAmount         big.Int
	JettonMaster          tongo.AccountID
	Success               bool
}

func (b BubbleAirdropClaim) ToAction() *Action {
	return &Action{
		AirdropClaim: &AirdropClaimAction{
			Distributor: b.Distributor,
			Recipient:   b.Recipient.Addr(),
			Claimed: assetTransfer{
				Amount:       b.ClaimedAmount,
				JettonMaster: b.JettonMaster,
				JettonWallet: b.RecipientJettonWallet,
			},
		},
		Type:    AirdropClaim,
		Success: b.Success,
	}
}

var AirdropClaimStraw = Straw[BubbleAirdropClaim]{
	CheckFuncs: []bubbleCheck{IsTx, HasOpcode(0x3a86f1a0)}, // todo use const from tongo.abi
	Builder: func(newAction *BubbleAirdropClaim, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Recipient = tx.inputFrom
		return nil
	},
	Children: []Straw[BubbleAirdropClaim]{
		{
			CheckFuncs: []bubbleCheck{Is(BubbleContractDeploy{})},
		},
		{
			CheckFuncs: []bubbleCheck{
				IsTx,
			},
			Builder: func(newAction *BubbleAirdropClaim, bubble *Bubble) error {
				tx := bubble.Info.(BubbleTx)
				newAction.Distributor = tx.account.Address
				return nil
			},
			SingleChild: &Straw[BubbleAirdropClaim]{
				CheckFuncs: []bubbleCheck{IsJettonTransfer},
				Builder: func(newAction *BubbleAirdropClaim, bubble *Bubble) error {
					tx := bubble.Info.(BubbleJettonTransfer)
					newAction.RecipientJettonWallet = tx.recipientWallet
					newAction.JettonMaster = tx.master
					newAction.ClaimedAmount = big.Int(tx.amount)
					newAction.Success = tx.success && newAction.Recipient.Address == tx.recipient.Address
					return nil
				},
			},
		},
	},
}
