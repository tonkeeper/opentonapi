package bath

import (
	"fmt"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
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
	CheckFuncs: []bubbleCheck{
		IsTx,
		Or(HasInterface(abi.AirdropInterlockerV1), HasInterface(abi.AirdropInterlockerV2)), // todo find trace with interface airdrop_interlocker_v2
		// todo add here operation with opcode 0x3a86f1a0
	},
	Builder: func(newAction *BubbleAirdropClaim, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Recipient = tx.inputFrom

		return nil
	},
	SingleChild: &Straw[BubbleAirdropClaim]{
		CheckFuncs: []bubbleCheck{
			IsTx,
		},
		Builder: func(newAction *BubbleAirdropClaim, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.Distributor = tx.account.Address

			return nil
		},
		SingleChild: &Straw[BubbleAirdropClaim]{
			CheckFuncs: []bubbleCheck{
				IsTx,
				HasInterface(abi.JettonWallet),
				HasOperation(abi.JettonTransferMsgOp),
			},
			SingleChild: &Straw[BubbleAirdropClaim]{
				CheckFuncs: []bubbleCheck{
					IsTx,
					HasInterface(abi.JettonWallet),
					HasOperation(abi.JettonInternalTransferMsgOp),
				},
				Builder: func(newAction *BubbleAirdropClaim, bubble *Bubble) error {
					tx := bubble.Info.(BubbleTx)
					newAction.RecipientJettonWallet = tx.account.Address
					body := tx.decodedBody.Value.(abi.JettonInternalTransferMsgBody)
					newAction.ClaimedAmount = big.Int(body.Amount)
					master, ok := tx.additionalInfo.JettonMaster(tx.account.Address)
					if !ok {
						return fmt.Errorf("cannot find jetton master")
					}
					newAction.JettonMaster = master

					return nil
				},
			},
		},
	},
}
