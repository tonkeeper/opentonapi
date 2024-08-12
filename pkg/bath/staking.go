package bath

import (
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/blockchain/config"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/ton"
)

type BubbleElectionsDepositStake struct {
	Staker  tongo.AccountID
	Amount  int64
	Success bool
}

func (ds BubbleElectionsDepositStake) ToAction() *Action {
	return &Action{
		ElectionsDepositStake: &ElectionsDepositStakeAction{
			Amount:  ds.Amount,
			Elector: config.ElectorAddress(),
			Staker:  ds.Staker,
		},
		Success: ds.Success,
		Type:    ElectionsDepositStake,
	}
}

var ElectionsDepositStakeStraw = Straw[BubbleElectionsDepositStake]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ElectorNewStakeMsgOp), IsAccount(config.ElectorAddress())},
	Builder: func(newAction *BubbleElectionsDepositStake, bubble *Bubble) error {
		bubbleTx := bubble.Info.(BubbleTx)
		newAction.Amount = bubbleTx.inputAmount
		newAction.Staker = bubbleTx.inputFrom.Address
		return nil
	},
	Children: []Straw[BubbleElectionsDepositStake]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ElectorNewStakeConfirmationMsgOp)},
			Builder: func(newAction *BubbleElectionsDepositStake, bubble *Bubble) error {
				newAction.Success = true
				return nil
			},
		},
	},
}

type BubbleElectionsRecoverStake struct {
	Staker  tongo.AccountID
	Amount  int64
	Success bool
}

func (b BubbleElectionsRecoverStake) ToAction() *Action {
	return &Action{
		ElectionsRecoverStake: &ElectionsRecoverStakeAction{
			Amount:  b.Amount,
			Elector: config.ElectorAddress(),
			Staker:  b.Staker,
		},
		Success: b.Success,
		Type:    ElectionsRecoverStake,
	}
}

var ElectionsRecoverStakeStraw = Straw[BubbleElectionsRecoverStake]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ElectorRecoverStakeRequestMsgOp), IsAccount(config.ElectorAddress())},
	Builder: func(newAction *BubbleElectionsRecoverStake, bubble *Bubble) error {
		bubbleTx := bubble.Info.(BubbleTx)
		newAction.Staker = bubbleTx.inputFrom.Address
		return nil
	},
	Children: []Straw[BubbleElectionsRecoverStake]{
		{
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.ElectorRecoverStakeResponseMsgOp)},
			Builder: func(newAction *BubbleElectionsRecoverStake, bubble *Bubble) error {
				newAction.Amount = bubble.Info.(BubbleTx).inputAmount
				newAction.Success = true
				return nil
			},
		},
	},
}

type BubbleDepositStake struct {
	Staker         tongo.AccountID
	Amount         int64
	Success        bool
	Pool           tongo.AccountID
	Implementation core.StakingImplementation
}

func (ds BubbleDepositStake) ToAction() *Action {
	return &Action{
		DepositStake: &DepositStakeAction{
			Amount:         ds.Amount,
			Pool:           ds.Pool,
			Staker:         ds.Staker,
			Implementation: ds.Implementation,
		},
		Success: ds.Success,
		Type:    DepositStake,
	}
}

var DepositTFStakeStraw = Straw[BubbleDepositStake]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.TvPool), HasTextComment("d")},
	Builder: func(newAction *BubbleDepositStake, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Pool = tx.account.Address
		newAction.Staker = tx.inputFrom.Address
		newAction.Amount = tx.inputAmount
		newAction.Success = tx.success
		newAction.Implementation = core.StakingImplementationTF
		return nil
	},
	SingleChild: &Straw[BubbleDepositStake]{
		CheckFuncs: []bubbleCheck{IsBounced},
		Optional:   true,
	},
}

type BubbleWithdrawStakeRequest struct {
	Staker         tongo.AccountID
	Amount         *int64
	Success        bool
	Pool           tongo.AccountID
	Implementation core.StakingImplementation
	attachedAmount int64 //
}

func (ds BubbleWithdrawStakeRequest) ToAction() *Action {
	return &Action{
		WithdrawStakeRequest: &WithdrawStakeRequestAction{
			Amount:         ds.Amount,
			Pool:           ds.Pool,
			Staker:         ds.Staker,
			Implementation: ds.Implementation,
		},
		Success: ds.Success,
		Type:    WithdrawStakeRequest,
	}
}

var WithdrawTFStakeRequestStraw = Straw[BubbleWithdrawStakeRequest]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.TvPool), HasTextComment("w")},
	Builder: func(newAction *BubbleWithdrawStakeRequest, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Pool = tx.account.Address
		newAction.Staker = tx.inputFrom.Address
		newAction.Success = tx.success
		newAction.attachedAmount = tx.inputAmount //maybe should extract fee but it is not important
		newAction.Implementation = core.StakingImplementationTF
		return nil
	},
	Children: []Straw[BubbleWithdrawStakeRequest]{
		{
			Optional:   true,
			CheckFuncs: []bubbleCheck{IsTx, AmountInterval(0, int64(ton.OneTON))},
		},
	},
}

type BubbleWithdrawStake struct {
	Staker         tongo.AccountID
	Amount         int64
	Pool           tongo.AccountID
	Implementation core.StakingImplementation
}

func (ds BubbleWithdrawStake) ToAction() *Action {
	return &Action{
		WithdrawStake: &WithdrawStakeAction{
			Amount:         ds.Amount,
			Pool:           ds.Pool,
			Staker:         ds.Staker,
			Implementation: ds.Implementation,
		},
		Success: true,
		Type:    WithdrawStake,
	}
}

var WithdrawStakeImmediatelyStraw = Straw[BubbleWithdrawStake]{
	CheckFuncs: []bubbleCheck{Is(BubbleWithdrawStakeRequest{})},
	Builder: func(newAction *BubbleWithdrawStake, bubble *Bubble) error {
		req := bubble.Info.(BubbleWithdrawStakeRequest)
		newAction.Pool = req.Pool
		newAction.Staker = req.Staker
		newAction.Amount = -req.attachedAmount
		newAction.Implementation = req.Implementation
		return nil
	},
	SingleChild: &Straw[BubbleWithdrawStake]{
		CheckFuncs: []bubbleCheck{IsTx, AmountInterval(int64(ton.OneTON), 1<<63-1)},
		Builder: func(newAction *BubbleWithdrawStake, bubble *Bubble) error {
			newAction.Amount += bubble.Info.(BubbleTx).inputAmount
			return nil
		},
	},
}

var DepositLiquidStakeStraw = Straw[BubbleDepositStake]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.TonstakePoolDepositMsgOp)}, //todo: check interface HasInterface(abi.TonstakePool),
	Builder: func(newAction *BubbleDepositStake, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Pool = tx.account.Address
		newAction.Staker = tx.inputFrom.Address
		newAction.Amount = tx.inputAmount - int64(ton.OneTON)
		if newAction.Amount < 0 {
			newAction.Amount = 0
		}
		newAction.Success = tx.success
		newAction.Implementation = core.StakingImplementationLiquidTF
		return nil
	},
	SingleChild: &Straw[BubbleDepositStake]{
		CheckFuncs: []bubbleCheck{IsBounced},
		Optional:   true,
	},
}

var WithdrawLiquidStake = Straw[BubbleWithdrawStake]{
	CheckFuncs: []bubbleCheck{Is(BubbleWithdrawStakeRequest{})},
	Builder: func(newAction *BubbleWithdrawStake, bubble *Bubble) error {
		request := bubble.Info.(BubbleWithdrawStakeRequest)
		newAction.Amount -= request.attachedAmount
		newAction.Pool = request.Pool
		newAction.Staker = request.Staker
		newAction.Implementation = request.Implementation
		return nil
	},
	SingleChild: &Straw[BubbleWithdrawStake]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.TonstakePoolWithdrawalMsgOp)},
		Builder: func(newAction *BubbleWithdrawStake, bubble *Bubble) error {
			newAction.Amount += bubble.Info.(BubbleTx).inputAmount
			return nil
		},
	},
}

var PendingWithdrawRequestLiquidStraw = Straw[BubbleWithdrawStakeRequest]{
	CheckFuncs: []bubbleCheck{Is(BubbleJettonBurn{})},
	Builder: func(newAction *BubbleWithdrawStakeRequest, bubble *Bubble) error {
		newAction.Staker = bubble.Info.(BubbleJettonBurn).sender.Address
		newAction.Implementation = core.StakingImplementationLiquidTF
		amount := big.Int(bubble.Info.(BubbleJettonBurn).amount)
		i := amount.Int64()
		newAction.Amount = &i
		return nil
	},
	SingleChild: &Straw[BubbleWithdrawStakeRequest]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.TonstakePoolWithdrawMsgOp)},
		Builder: func(newAction *BubbleWithdrawStakeRequest, bubble *Bubble) error {
			newAction.Pool = bubble.Info.(BubbleTx).account.Address
			newAction.attachedAmount = bubble.Info.(BubbleTx).inputAmount
			return nil
		},
		SingleChild: &Straw[BubbleWithdrawStakeRequest]{
			Optional:   true,
			CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.TonstakePayoutMintJettonsMsgOp)},
			SingleChild: &Straw[BubbleWithdrawStakeRequest]{
				CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.TonstakeNftInitMsgOp)},
				SingleChild: &Straw[BubbleWithdrawStakeRequest]{
					CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.NftOwnershipAssignedMsgOp)},
					Builder: func(newAction *BubbleWithdrawStakeRequest, bubble *Bubble) error {
						newAction.Success = true
						return nil
					},
				},
			},
		},
	},
}
