package bath

import (
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
)

const (
	Empty            ActionType = "Empty"
	TonTransfer      ActionType = "TonTransfer"
	NftTransfer      ActionType = "NftItemTransfer"
	JettonTransfer   ActionType = "JettonTransfer"
	ContractDeploy   ActionType = "ContractDeploy"
	Subscription     ActionType = "Subscribe"
	UnSubscription   ActionType = "UnSubscribe"
	AuctionBid       ActionType = "AuctionBid"
	AuctionTgInitBid ActionType = "AuctionTgInitBid"

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
		TonTransfer    *TonTransferAction
		NftTransfer    *NftTransferAction
		JettonTransfer *JettonTransferAction
		ContractDeploy *ContractDeployAction
		Subscription   *SubscriptionAction
		UnSubscription *UnSubscriptionAction
		AuctionBid     *AuctionBidAction
		Success        bool
		Type           ActionType
	}
	TonTransferAction struct {
		Amount    int64
		Comment   *string
		Payload   *string
		Recipient tongo.AccountID
		Sender    tongo.AccountID
		Refund    *Refund
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
		Amount           decimal.Decimal
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
