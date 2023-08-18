package bath

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"golang.org/x/exp/slices"
)

const (
	TonTransfer       ActionType = "TonTransfer"
	SmartContractExec ActionType = "SmartContractExec"
	NftItemTransfer   ActionType = "NftItemTransfer"
	NftPurchase       ActionType = "NftPurchase"
	JettonTransfer    ActionType = "JettonTransfer"
	JettonMint        ActionType = "JettonMint"
	JettonBurn        ActionType = "JettonBurn"
	ContractDeploy    ActionType = "ContractDeploy"
	Subscription      ActionType = "Subscribe"
	UnSubscription    ActionType = "UnSubscribe"
	DepositStake      ActionType = "DepositStake"
	RecoverStake      ActionType = "RecoverStake"
	JettonSwap        ActionType = "JettonSwap"
	AuctionBid        ActionType = "AuctionBid"

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
	EncryptedComment struct {
		EncryptionType string
		CipherText     []byte
	}

	Action struct {
		TonTransfer       *TonTransferAction    `json:",omitempty"`
		SmartContractExec *SmartContractAction  `json:",omitempty"`
		NftItemTransfer   *NftTransferAction    `json:",omitempty"`
		NftPurchase       *NftPurchaseAction    `json:",omitempty"`
		JettonTransfer    *JettonTransferAction `json:",omitempty"`
		JettonMint        *JettonMintAction     `json:",omitempty"`
		JettonBurn        *JettonBurnAction     `json:",omitempty"`
		ContractDeploy    *ContractDeployAction `json:",omitempty"`
		Subscription      *SubscriptionAction   `json:",omitempty"`
		UnSubscription    *UnSubscriptionAction `json:",omitempty"`
		AuctionBid        *AuctionBidAction     `json:",omitempty"`
		DepositStake      *DepositStakeAction   `json:",omitempty"`
		RecoverStake      *RecoverStakeAction   `json:",omitempty"`
		JettonSwap        *JettonSwapAction     `json:",omitempty"`
		Success           bool
		Type              ActionType
	}
	TonTransferAction struct {
		Amount           int64
		Comment          *string
		EncryptedComment *EncryptedComment
		Recipient        tongo.AccountID
		Sender           tongo.AccountID
		Refund           *Refund
	}
	SmartContractAction struct {
		TonAttached int64
		Executor    tongo.AccountID
		Contract    tongo.AccountID
		Operation   string
		Payload     string
	}

	NftTransferAction struct {
		Comment          *string
		EncryptedComment *EncryptedComment
		Recipient        *tongo.AccountID
		Sender           *tongo.AccountID
		Nft              tongo.AccountID
		Refund           *Refund
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
		EncryptedComment *EncryptedComment
		Jetton           tongo.AccountID
		Recipient        *tongo.AccountID
		Sender           *tongo.AccountID
		RecipientsWallet tongo.AccountID
		SendersWallet    tongo.AccountID
		Amount           tlb.VarUInteger16
		Refund           *Refund
	}

	JettonMintAction struct {
		Jetton           tongo.AccountID
		Recipient        *tongo.AccountID
		RecipientsWallet tongo.AccountID
		Amount           tlb.VarUInteger16
	}

	JettonBurnAction struct {
		Jetton        tongo.AccountID
		Sender        *tongo.AccountID
		SendersWallet tongo.AccountID
		Amount        tlb.VarUInteger16
	}

	ContractDeployAction struct {
		Address    tongo.AccountID
		Interfaces []abi.ContractInterface
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

	JettonSwapAction struct {
		Dex             Dex
		UserWallet      tongo.AccountID
		Router          tongo.AccountID
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

func detectDirection(a, from, to tongo.AccountID, amount int64) int64 {
	switch {
	case from == to:
		return 0
	case from == a:
		return -amount
	case to == a:
		return amount
	}
	return 0
}

func (a Action) ContributeToExtra(account tongo.AccountID) int64 {
	switch a.Type {
	case JettonTransfer, NftItemTransfer, ContractDeploy, UnSubscription, JettonSwap: // actions without extra
		return 0
	case TonTransfer:
		return detectDirection(account, a.TonTransfer.Sender, a.TonTransfer.Recipient, a.TonTransfer.Amount)
	case SmartContractExec:
		return detectDirection(account, a.SmartContractExec.Executor, a.SmartContractExec.Contract, a.SmartContractExec.TonAttached)
	case NftPurchase:
		return detectDirection(account, a.NftPurchase.Buyer, a.NftPurchase.Seller, a.NftPurchase.Price)
	case AuctionBid:
		return detectDirection(account, a.AuctionBid.Bidder, a.AuctionBid.Auction, a.AuctionBid.Amount)
	case DepositStake:
		return detectDirection(account, a.DepositStake.Staker, a.DepositStake.Elector, a.DepositStake.Amount)
	case RecoverStake:
		return detectDirection(account, a.RecoverStake.Elector, a.RecoverStake.Staker, a.RecoverStake.Amount)
	case Subscription:
		return detectDirection(account, a.Subscription.Subscriber, a.Subscription.Beneficiary, a.Subscription.Amount)
	default:
		panic("unknown action type")
	}
}

func (a Action) IsSubject(account tongo.AccountID) bool {
	for _, i := range []interface{ SubjectAccounts() []tongo.AccountID }{
		a.TonTransfer,
		a.SmartContractExec,
		a.NftItemTransfer,
		a.NftPurchase,
		a.JettonTransfer,
		a.ContractDeploy,
		a.Subscription,
		a.UnSubscription,
		a.AuctionBid,
		a.DepositStake,
		a.RecoverStake,
		a.JettonSwap,
	} {
		if i != nil && !reflect.ValueOf(i).IsNil() {
			return slices.Contains(i.SubjectAccounts(), account)
		}
	}
	return false
}

func (a *TonTransferAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Sender, a.Recipient}
}

func (a *SmartContractAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Contract, a.Executor}
}

func (a *NftTransferAction) SubjectAccounts() []tongo.AccountID {
	accounts := make([]tongo.AccountID, 0, 2)
	if a.Sender != nil {
		accounts = append(accounts, *a.Sender)
	}
	if a.Recipient != nil {
		accounts = append(accounts, *a.Recipient)
	}
	return accounts
}

func (a *JettonTransferAction) SubjectAccounts() []tongo.AccountID {
	accounts := make([]tongo.AccountID, 0, 2)
	if a.Sender != nil {
		accounts = append(accounts, *a.Sender)
	}
	if a.Recipient != nil {
		accounts = append(accounts, *a.Recipient)
	}
	return accounts
}

func (a *SubscriptionAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Subscriber, a.Beneficiary}
}

func (a *NftPurchaseAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Buyer, a.Seller}
}

func (a *AuctionBidAction) SubjectAccounts() []tongo.AccountID {
	accounts := make([]tongo.AccountID, 0, 2)
	accounts = append(accounts, a.Bidder)
	return accounts
}
func (a *ContractDeployAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Address}
}

func (a *JettonSwapAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.UserWallet}
}
func (a *UnSubscriptionAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Subscriber, a.Beneficiary}
}
func (a *DepositStakeAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Staker}
}

func (a *RecoverStakeAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Staker}
}
