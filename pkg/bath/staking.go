package bath

import (
	"github.com/tonkeeper/opentonapi/pkg/blockchain/config"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
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
	Staker  tongo.AccountID
	Amount  int64
	Success bool
	Pool    tongo.AccountID
}

func (ds BubbleDepositStake) ToAction() *Action {
	return &Action{
		DepositStake: &DepositStakeAction{
			Amount: ds.Amount,
			Pool:   ds.Pool,
			Staker: ds.Staker,
		},
		Success: ds.Success,
		Type:    DepositStake,
	}
}

var DepositTFStakeStraw = Straw[BubbleDepositStake]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.TfNominator), HasTextComment("d")},
	Builder: func(newAction *BubbleDepositStake, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Pool = tx.account.Address
		newAction.Staker = tx.inputFrom.Address
		newAction.Amount = tx.inputAmount
		newAction.Success = tx.success
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
	attachedAmount int64 //
}

func (ds BubbleWithdrawStakeRequest) ToAction() *Action {
	return &Action{
		WithdrawStakeRequest: &WithdrawStakeRequestAction{
			Amount: ds.Amount,
			Pool:   ds.Pool,
			Staker: ds.Staker,
		},
		Success: ds.Success,
		Type:    WithdrawStakeRequest,
	}
}

var WithdrawTFStakeRequestStraw = Straw[BubbleWithdrawStakeRequest]{
	CheckFuncs: []bubbleCheck{IsTx, HasInterface(abi.TfNominator), HasTextComment("w")},
	Builder: func(newAction *BubbleWithdrawStakeRequest, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Pool = tx.account.Address
		newAction.Staker = tx.inputFrom.Address
		newAction.Success = tx.success
		newAction.attachedAmount = tx.inputAmount //maybe should extract fee but it is not important
		return nil
	},
	Children: []Straw[BubbleWithdrawStakeRequest]{
		{
			Optional:   true,
			CheckFuncs: []bubbleCheck{IsTx, AmountInterval(0, int64(tongo.OneTON))},
		},
	},
}

type BubbleWithdrawStake struct {
	Staker tongo.AccountID
	Amount int64
	Pool   tongo.AccountID
}

func (ds BubbleWithdrawStake) ToAction() *Action {
	return &Action{
		WithdrawStake: &WithdrawStakeAction{
			Amount: ds.Amount,
			Pool:   ds.Pool,
			Staker: ds.Staker,
		},
		Type: WithdrawStake,
	}
}

var WithdrawStakeImmediatelyStraw = Straw[BubbleWithdrawStake]{
	CheckFuncs: []bubbleCheck{Is(BubbleWithdrawStakeRequest{})},
	Builder: func(newAction *BubbleWithdrawStake, bubble *Bubble) error {
		req := bubble.Info.(BubbleWithdrawStakeRequest)
		newAction.Pool = req.Pool
		newAction.Staker = req.Staker
		newAction.Amount = -req.attachedAmount
		return nil
	},
	SingleChild: &Straw[BubbleWithdrawStake]{
		CheckFuncs: []bubbleCheck{IsTx, AmountInterval(int64(tongo.OneTON), 1<<63-1)},
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
		newAction.Amount = tx.inputAmount - int64(tongo.OneTON)
		newAction.Success = tx.success
		return nil
	},
	SingleChild: &Straw[BubbleDepositStake]{
		CheckFuncs: []bubbleCheck{IsBounced},
		Optional:   true,
	},
}
