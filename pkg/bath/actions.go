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
	Empty              ActionType = "Empty"
	TonTransfer        ActionType = "TonTransfer"
	SmartContractExec  ActionType = "SmartContractExec"
	NftItemTransfer    ActionType = "NftItemTransfer"
	GetGemsNftPurchase ActionType = "GetGemsNftPurchase"
	JettonTransfer     ActionType = "JettonTransfer"
	ContractDeploy     ActionType = "ContractDeploy"
	Subscription       ActionType = "Subscription"
	UnSubscription     ActionType = "UnSubscribe"
	AuctionBid         ActionType = "AuctionBid"
	AuctionTgInitBid   ActionType = "AuctionTgInitBid"

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
	// SimplePreview shortly describes what an action is about.
	SimplePreview struct {
		Name     string
		Value    string
		Accounts []tongo.AccountID
		// MessageID specifies a message's ID from pkg/i18n/translation/*.toml
		MessageID    string
		TemplateData map[string]interface{}
	}
	Action struct {
		TonTransfer        *TonTransferAction
		SmartContractExec  *SmartContractAction
		NftItemTransfer    *NftTransferAction
		GetGemsNftPurchase *GetGemsNftPurchaseAction
		JettonTransfer     *JettonTransferAction
		ContractDeploy     *ContractDeployAction
		Subscription       *SubscriptionAction
		UnSubscription     *UnSubscriptionAction
		AuctionBid         *AuctionBidAction
		Success            bool
		Type               ActionType
		SimplePreview      SimplePreview
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

	GetGemsNftPurchaseAction struct {
		Nft      tongo.AccountID
		NewOwner tongo.AccountID
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

func CollectActionsAndValueFlow(bubble *Bubble, forAccount *tongo.AccountID, book addressBook) ([]Action, *ValueFlow) {
	var actions []Action
	valueFlow := newValueFlow()
	if forAccount == nil || slices.Contains(bubble.Accounts, *forAccount) {
		a := bubble.Info.ToAction(book)
		if a != nil {
			actions = append(actions, *a)
		}
	}
	for _, c := range bubble.Children {
		childActions, childValueFlow := CollectActionsAndValueFlow(c, forAccount, book)
		actions = append(actions, childActions...)
		valueFlow.Merge(childValueFlow)
	}
	valueFlow.Merge(bubble.ValueFlow)
	return actions, valueFlow
}
