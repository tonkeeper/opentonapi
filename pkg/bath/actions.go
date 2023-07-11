package bath

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
	"golang.org/x/exp/slices"

	"github.com/tonkeeper/opentonapi/pkg/core"
)

const (
	Empty             ActionType = "Empty"
	TonTransfer       ActionType = "TonTransfer"
	SmartContractExec ActionType = "SmartContractExec"
	NftItemTransfer   ActionType = "NftItemTransfer"
	NftPurchase       ActionType = "NftPurchase"
	JettonTransfer    ActionType = "JettonTransfer"
	ContractDeploy    ActionType = "ContractDeploy"
	Subscription      ActionType = "Subscribe"
	UnSubscription    ActionType = "UnSubscribe"
	DepositStake      ActionType = "DepositStake"
	RecoverStake      ActionType = "RecoverStake"
	STONfiSwap        ActionType = "STONfiSwap"
	AuctionBid        ActionType = "AuctionBid"
	AuctionTgInitBid  ActionType = "AuctionTgInitBid"

	RefundDnsTg   RefundType = "DNS.tg"
	RefundDnsTon  RefundType = "DNS.ton"
	RefundGetGems RefundType = "GetGems"
	RefundUnknown RefundType = "unknown"
)

type ActionType string
type RefundType string

type NftAuctionType string

const (
	GetGemsAuction NftAuctionType = "getgems"
	BasicAuction   NftAuctionType = "basic"
)

type (
	Refund struct {
		Type   RefundType
		Origin string
	}

	Action struct {
		TonTransfer       *TonTransferAction    `json:",omitempty"`
		SmartContractExec *SmartContractAction  `json:",omitempty"`
		NftItemTransfer   *NftTransferAction    `json:",omitempty"`
		NftPurchase       *NftPurchaseAction    `json:",omitempty"`
		JettonTransfer    *JettonTransferAction `json:",omitempty"`
		ContractDeploy    *ContractDeployAction `json:",omitempty"`
		Subscription      *SubscriptionAction   `json:",omitempty"`
		UnSubscription    *UnSubscriptionAction `json:",omitempty"`
		AuctionBid        *AuctionBidAction     `json:",omitempty"`
		DepositStake      *DepositStakeAction   `json:",omitempty"`
		RecoverStake      *RecoverStakeAction   `json:",omitempty"`
		STONfiSwap        *STONfiSwapAction     `json:",omitempty"`
		Success           bool
		Type              ActionType
	}
	TonTransferAction struct {
		Amount    int64
		Comment   *string
		Recipient tongo.AccountID
		Sender    tongo.AccountID
		Refund    *Refund
	}
	SmartContractAction struct {
		TonAttached int64
		Executor    tongo.AccountID
		Contract    tongo.AccountID
		Operation   string
		Payload     string
	}

	NftTransferAction struct {
		Comment   *string
		Recipient *tongo.AccountID
		Sender    *tongo.AccountID
		Nft       tongo.AccountID
		Refund    *Refund
	}

	NftPurchaseAction struct {
		Nft         tongo.AccountID
		Buyer       tongo.AccountID
		Seller      tongo.AccountID
		AuctionType NftAuctionType
		Price       int64
	}

	DepositStakeAction struct {
		Amount  int64
		Elector tongo.AccountID
		Staker  tongo.AccountID
	}

	RecoverStakeAction struct {
		Amount  int64
		Elector tongo.AccountID
		Staker  tongo.AccountID
	}

	JettonTransferAction struct {
		Comment          *string
		Jetton           tongo.AccountID
		Recipient        *tongo.AccountID
		Sender           *tongo.AccountID
		RecipientsWallet tongo.AccountID
		SendersWallet    tongo.AccountID
		Amount           tlb.VarUInteger16
		Refund           *Refund
	}

	ContractDeployAction struct {
		Address    tongo.AccountID
		Interfaces []string
	}

	SubscriptionAction struct {
		Subscription tongo.AccountID
		Subscriber   tongo.AccountID
		Beneficiary  tongo.AccountID
		Amount       int64
		First        bool
	}

	UnSubscriptionAction struct {
		Subscription tongo.AccountID
		Subscriber   tongo.AccountID
		Beneficiary  tongo.AccountID
	}

	AuctionBidAction struct {
		Bidder         tongo.AccountID
		PreviousBidder *tongo.AccountID
		Bid            int64
		Item           *core.NftItem
		AuctionType    string
	}

	STONfiSwapAction struct {
		UserWallet      tongo.AccountID
		STONfiRouter    tongo.AccountID
		JettonWalletIn  tongo.AccountID
		JettonMasterIn  tongo.AccountID
		JettonWalletOut tongo.AccountID
		JettonMasterOut tongo.AccountID
		AmountIn        uint64
		AmountOut       uint64
	}
)

func (a Action) String() string {
	val := reflect.ValueOf(a)
	f := val.FieldByName(string(a.Type))
	b, _ := json.Marshal(f.Interface())
	return fmt.Sprintf("%v: %+v", a.Type, string(b))
}

func CollectActionsAndValueFlow(bubble *Bubble, forAccount *tongo.AccountID) ([]Action, *ValueFlow) {
	var actions []Action
	valueFlow := newValueFlow()
	if forAccount == nil || slices.Contains(bubble.Accounts, *forAccount) {
		a := bubble.Info.ToAction()
		if a != nil {
			actions = append(actions, *a)
		}
	}
	for _, c := range bubble.Children {
		childActions, childValueFlow := CollectActionsAndValueFlow(c, forAccount)
		actions = append(actions, childActions...)
		valueFlow.Merge(childValueFlow)
	}
	valueFlow.Merge(bubble.ValueFlow)
	return actions, valueFlow
}

func (a Action) ContributeToExtra(account tongo.AccountID, extra int64) int64 {
	if a.TonTransfer != nil {
		return a.TonTransfer.ContributeToExtra(account, extra)
	}
	if a.SmartContractExec != nil {
		return a.SmartContractExec.ContributeToExtra(account, extra)
	}
	return extra
}

func (a *TonTransferAction) ContributeToExtra(account tongo.AccountID, extra int64) int64 {
	if a.Sender == account {
		return extra - a.Amount
	}
	if a.Recipient == account {
		return extra + a.Amount
	}
	return extra
}

func (a *SmartContractAction) ContributeToExtra(account tongo.AccountID, extra int64) int64 {
	if a.Executor == account {
		return extra - a.TonAttached
	}
	if a.Contract == account {
		return extra + a.TonAttached
	}
	return extra
}
