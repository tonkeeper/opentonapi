package bath

import (
	"encoding/json"
	"fmt"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"reflect"
)

const (
	Empty             ActionType = "Empty"
	TonTransfer       ActionType = "TonTransfer"
	SmartContractExec ActionType = "SmartContractExec"
	NftItemTransfer   ActionType = "NftItemTransfer"
	JettonTransfer    ActionType = "JettonTransfer"
	ContractDeploy    ActionType = "ContractDeploy"
	Subscription      ActionType = "Subscribe"
	UnSubscription    ActionType = "UnSubscribe"
	AuctionBid        ActionType = "AuctionBid"
	AuctionTgInitBid  ActionType = "AuctionTgInitBid"

	RefundDnsTg   RefundType = "DNS.tg"
	RefundDnsTon  RefundType = "DNS.ton"
	RefundGetGems RefundType = "GetGems"
	RefundUnknown RefundType = "unknown"
)

type ActionType string
type RefundType string

type (
	Refund struct {
		Type   RefundType
		Origin string
	}

	Action struct {
		TonTransfer       *TonTransferAction
		SmartContractExec *SmartContractAction
		NftItemTransfer   *NftTransferAction
		JettonTransfer    *JettonTransferAction
		ContractDeploy    *ContractDeployAction
		Subscription      *SubscriptionAction
		UnSubscription    *UnSubscriptionAction
		AuctionBid        *AuctionBidAction
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
		Sender     tongo.AccountID
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
)

func (a Action) String() string {
	val := reflect.ValueOf(a)
	f := val.FieldByName(string(a.Type))
	b, _ := json.Marshal(f.Interface())
	return fmt.Sprintf("%v: %+v", a.Type, string(b))
}

func (f Fee) String() string {
	return fmt.Sprintf("%v: %v/%v/%v/%v/%v", f.WhoPay, f.Total(), f.Storage, f.Compute, f.Deposit, f.Refund)
}

func CollectActions(bubble *Bubble, forAccount *tongo.AccountID) ([]Action, []Fee) {
	fees := make(map[tongo.AccountID]Fee)
	actions := collectActions(bubble, fees, forAccount)
	return actions, maps.Values(fees)
}

func collectActions(bubble *Bubble, fees map[tongo.AccountID]Fee, forAccount *tongo.AccountID) []Action {
	var actions []Action
	if forAccount == nil || slices.Contains(bubble.Accounts, *forAccount) {
		a := bubble.Info.ToAction()
		if a != nil {
			actions = append(actions, *a)
		}
	}
	for _, c := range bubble.Children {
		a := collectActions(c, fees, forAccount)
		actions = append(actions, a...)
	}
	f := fees[bubble.Fee.WhoPay]
	bubble.Fee.Add(f)
	fees[bubble.Fee.WhoPay] = bubble.Fee
	return actions
}
