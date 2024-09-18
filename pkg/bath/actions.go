package bath

import (
	"cmp"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo/ton"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"golang.org/x/exp/slices"
)

const (
	TonTransfer           ActionType = "TonTransfer"
	SmartContractExec     ActionType = "SmartContractExec"
	NftItemTransfer       ActionType = "NftItemTransfer"
	NftPurchase           ActionType = "NftPurchase"
	JettonTransfer        ActionType = "JettonTransfer"
	JettonMint            ActionType = "JettonMint"
	JettonBurn            ActionType = "JettonBurn"
	ContractDeploy        ActionType = "ContractDeploy"
	Subscription          ActionType = "Subscribe"
	UnSubscription        ActionType = "UnSubscribe"
	ElectionsDepositStake ActionType = "ElectionsDepositStake"
	ElectionsRecoverStake ActionType = "ElectionsRecoverStake"
	DepositStake          ActionType = "DepositStake"
	WithdrawStake         ActionType = "WithdrawStake"
	WithdrawStakeRequest  ActionType = "WithdrawStakeRequest"
	JettonSwap            ActionType = "JettonSwap"
	AuctionBid            ActionType = "AuctionBid"
	DomainRenew           ActionType = "DomainRenew"
	InscriptionMint       ActionType = "InscriptionMint"
	InscriptionTransfer   ActionType = "InscriptionTransfer"

	RefundDnsTg   RefundType = "DNS.tg"
	RefundDnsTon  RefundType = "DNS.ton"
	RefundGetGems RefundType = "GetGems"
	RefundUnknown RefundType = "unknown"
)

type ActionType string
type RefundType string

type NftAuctionType string

const (
	GetGemsAuction  NftAuctionType = "getgems"
	DnsTonAuction                  = "DNS.ton"
	DnsTgAuction                   = "DNS.tg"
	NumberTgAuction                = "NUMBER.tg"
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
		TonTransfer           *TonTransferAction           `json:",omitempty"`
		SmartContractExec     *SmartContractAction         `json:",omitempty"`
		NftItemTransfer       *NftTransferAction           `json:",omitempty"`
		NftPurchase           *NftPurchaseAction           `json:",omitempty"`
		JettonTransfer        *JettonTransferAction        `json:",omitempty"`
		JettonMint            *JettonMintAction            `json:",omitempty"`
		JettonBurn            *JettonBurnAction            `json:",omitempty"`
		ContractDeploy        *ContractDeployAction        `json:",omitempty"`
		Subscription          *SubscriptionAction          `json:",omitempty"`
		UnSubscription        *UnSubscriptionAction        `json:",omitempty"`
		AuctionBid            *AuctionBidAction            `json:",omitempty"`
		ElectionsDepositStake *ElectionsDepositStakeAction `json:",omitempty"`
		ElectionsRecoverStake *ElectionsRecoverStakeAction `json:",omitempty"`
		DepositStake          *DepositStakeAction          `json:",omitempty"`
		WithdrawStake         *WithdrawStakeAction         `json:",omitempty"`
		WithdrawStakeRequest  *WithdrawStakeRequestAction  `json:",omitempty"`
		JettonSwap            *JettonSwapAction            `json:",omitempty"`
		DnsRenew              *DnsRenewAction              `json:",omitempty"`
		InscriptionMint       *InscriptionMintAction       `json:",omitempty"`
		InscriptionTransfer   *InscriptionTransferAction   `json:",omitempty"`
		Success               bool
		Type                  ActionType
		Error                 *string `json:",omitempty"`
		BaseTransactions      []ton.Bits256
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

	ElectionsDepositStakeAction struct {
		Amount  int64
		Elector tongo.AccountID
		Staker  tongo.AccountID
	}

	ElectionsRecoverStakeAction struct {
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
		isWrappedTon     bool
	}

	JettonMintAction struct {
		Jetton           tongo.AccountID
		Recipient        tongo.AccountID
		RecipientsWallet tongo.AccountID
		Amount           tlb.VarUInteger16
	}

	JettonBurnAction struct {
		Jetton        tongo.AccountID
		Sender        tongo.AccountID
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
		Dex        Dex
		UserWallet tongo.AccountID
		Router     tongo.AccountID
		In         assetTransfer
		Out        assetTransfer
	}
	DepositStakeAction struct {
		Staker         tongo.AccountID
		Amount         int64
		Pool           tongo.AccountID
		Implementation core.StakingImplementation
	}
	WithdrawStakeAction struct {
		Staker         tongo.AccountID
		Amount         int64
		Pool           tongo.AccountID
		Implementation core.StakingImplementation
	}
	WithdrawStakeRequestAction struct {
		Staker         tongo.AccountID
		Amount         *int64
		Pool           tongo.AccountID
		Implementation core.StakingImplementation
	}
	assetTransfer struct {
		Amount       big.Int
		IsTon        bool
		JettonMaster tongo.AccountID
		JettonWallet tongo.AccountID
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
			slices.SortFunc(bubble.Transaction, func(a, b ton.Bits256) int {
				return cmp.Compare(binary.NativeEndian.Uint64(a[:8]), binary.NativeEndian.Uint64(b[:8])) //just for compact
			})
			a.BaseTransactions = slices.Compact(bubble.Transaction)
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
	if !a.Success {
		return 0
	}
	switch a.Type {
	case NftItemTransfer, ContractDeploy, UnSubscription, JettonMint, JettonBurn, WithdrawStakeRequest, DomainRenew, InscriptionMint, InscriptionTransfer: // actions without extra
		return 0
	case TonTransfer:
		return detectDirection(account, a.TonTransfer.Sender, a.TonTransfer.Recipient, a.TonTransfer.Amount)
	case SmartContractExec:
		return detectDirection(account, a.SmartContractExec.Executor, a.SmartContractExec.Contract, a.SmartContractExec.TonAttached)
	case NftPurchase:
		return detectDirection(account, a.NftPurchase.Buyer, a.NftPurchase.Seller, a.NftPurchase.Price)
	case AuctionBid:
		return detectDirection(account, a.AuctionBid.Bidder, a.AuctionBid.Auction, a.AuctionBid.Amount)
	case ElectionsDepositStake:
		return detectDirection(account, a.ElectionsDepositStake.Staker, a.ElectionsDepositStake.Elector, a.ElectionsDepositStake.Amount)
	case ElectionsRecoverStake:
		return detectDirection(account, a.ElectionsRecoverStake.Elector, a.ElectionsRecoverStake.Staker, a.ElectionsRecoverStake.Amount)
	case Subscription:
		return detectDirection(account, a.Subscription.Subscriber, a.Subscription.Beneficiary, a.Subscription.Amount)
	case DepositStake:
		return detectDirection(account, a.DepositStake.Staker, a.DepositStake.Pool, a.DepositStake.Amount)
	case JettonTransfer:
		if !a.JettonTransfer.isWrappedTon {
			return 0
		}
		if a.JettonTransfer.Sender == nil || a.JettonTransfer.Recipient == nil { //some fucking shit - we can't detect how to interpretate it
			return 0
		}
		b := big.Int(a.JettonTransfer.Amount)
		return detectDirection(account, *a.JettonTransfer.Sender, *a.JettonTransfer.Recipient, b.Int64())
	case JettonSwap:
		if account != a.JettonSwap.UserWallet {
			return 0
		}
		if a.JettonSwap.In.IsTon {
			return -a.JettonSwap.In.Amount.Int64()
		}
		if a.JettonSwap.Out.IsTon {
			return a.JettonSwap.Out.Amount.Int64()
		}
		return 0
	case WithdrawStake:
		return detectDirection(account, a.WithdrawStake.Pool, a.WithdrawStake.Staker, a.WithdrawStake.Amount)
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
		a.ElectionsDepositStake,
		a.ElectionsRecoverStake,
		a.DepositStake,
		a.WithdrawStake,
		a.WithdrawStakeRequest,
		a.JettonSwap,
		a.JettonMint,
		a.JettonBurn,
		a.DnsRenew,
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
func (a *ElectionsDepositStakeAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Staker}
}

func (a *ElectionsRecoverStakeAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Staker}
}

func (a *JettonBurnAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Sender}
}

func (a *JettonMintAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Recipient}
}

func (a *DepositStakeAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Staker, a.Pool}
}

func (a *WithdrawStakeAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Staker, a.Pool}
}
func (a *WithdrawStakeRequestAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Staker, a.Pool}
}
