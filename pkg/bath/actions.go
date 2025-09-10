package bath

import (
	"cmp"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"

	"github.com/google/uuid"
	"github.com/tonkeeper/opentonapi/pkg/references"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo/ton"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"golang.org/x/exp/slices"
)

const (
	TonTransfer               ActionType = "TonTransfer"
	ExtraCurrencyTransfer     ActionType = "ExtraCurrencyTransfer"
	SmartContractExec         ActionType = "SmartContractExec"
	GasRelay                  ActionType = "GasRelay"
	NftItemTransfer           ActionType = "NftItemTransfer"
	NftPurchase               ActionType = "NftPurchase"
	JettonTransfer            ActionType = "JettonTransfer"
	JettonMint                ActionType = "JettonMint"
	JettonBurn                ActionType = "JettonBurn"
	ContractDeploy            ActionType = "ContractDeploy"
	Subscribe                 ActionType = "Subscribe"
	UnSubscribe               ActionType = "UnSubscribe"
	ElectionsDepositStake     ActionType = "ElectionsDepositStake"
	ElectionsRecoverStake     ActionType = "ElectionsRecoverStake"
	DepositStake              ActionType = "DepositStake"
	WithdrawStake             ActionType = "WithdrawStake"
	WithdrawStakeRequest      ActionType = "WithdrawStakeRequest"
	DepositTokenStake         ActionType = "DepositTokenStake"
	WithdrawTokenStakeRequest ActionType = "WithdrawTokenStakeRequest"
	JettonSwap                ActionType = "JettonSwap"
	AuctionBid                ActionType = "AuctionBid"
	DomainRenew               ActionType = "DomainRenew"
	Purchase                  ActionType = "Purchase"
	AddExtension              ActionType = "AddExtension"
	RemoveExtension           ActionType = "RemoveExtension"
	SetSignatureAllowed       ActionType = "SetSignatureAllowed"
	Unknown                   ActionType = "Unknown"
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
		TonTransfer               *TonTransferAction               `json:",omitempty"`
		ExtraCurrencyTransfer     *ExtraCurrencyTransferAction     `json:",omitempty"`
		SmartContractExec         *SmartContractAction             `json:",omitempty"`
		GasRelay                  *GasRelayAction                  `json:",omitempty"`
		NftItemTransfer           *NftTransferAction               `json:",omitempty"`
		NftPurchase               *NftPurchaseAction               `json:",omitempty"`
		JettonTransfer            *JettonTransferAction            `json:",omitempty"`
		JettonMint                *JettonMintAction                `json:",omitempty"`
		JettonBurn                *JettonBurnAction                `json:",omitempty"`
		ContractDeploy            *ContractDeployAction            `json:",omitempty"`
		Subscribe                 *SubscribeAction                 `json:",omitempty"`
		UnSubscribe               *UnSubscribeAction               `json:",omitempty"`
		AuctionBid                *AuctionBidAction                `json:",omitempty"`
		ElectionsDepositStake     *ElectionsDepositStakeAction     `json:",omitempty"`
		ElectionsRecoverStake     *ElectionsRecoverStakeAction     `json:",omitempty"`
		DepositStake              *DepositStakeAction              `json:",omitempty"`
		WithdrawStake             *WithdrawStakeAction             `json:",omitempty"`
		WithdrawStakeRequest      *WithdrawStakeRequestAction      `json:",omitempty"`
		DepositTokenStake         *DepositTokenStakeAction         `json:",omitempty"`
		WithdrawTokenStakeRequest *WithdrawTokenStakeRequestAction `json:",omitempty"`
		JettonSwap                *JettonSwapAction                `json:",omitempty"`
		DnsRenew                  *DnsRenewAction                  `json:",omitempty"`
		Purchase                  *PurchaseAction                  `json:",omitempty"`
		AddExtension              *AddExtensionAction              `json:",omitempty"`
		RemoveExtension           *RemoveExtensionAction           `json:",omitempty"`
		SetSignatureAllowed       *SetSignatureAllowedAction       `json:",omitempty"`
		Success                   bool
		Type                      ActionType
		Error                     *string `json:",omitempty"`
		BaseTransactions          []ton.Bits256
	}
	TonTransferAction struct {
		Amount           int64
		Comment          *string
		EncryptedComment *EncryptedComment
		Recipient        tongo.AccountID
		Sender           tongo.AccountID
		Refund           *Refund
	}
	ExtraCurrencyTransferAction struct {
		CurrencyID       int32
		Amount           tlb.VarUInteger32
		Comment          *string
		EncryptedComment *EncryptedComment
		Recipient        tongo.AccountID
		Sender           tongo.AccountID
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
		Price       core.Price
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

	SubscribeAction struct {
		Subscription tongo.AccountID
		Subscriber   tongo.AccountID
		Admin        tongo.AccountID
		WithdrawTo   tongo.AccountID
		Price        core.Price
		First        bool
	}

	UnSubscribeAction struct {
		Subscription tongo.AccountID
		Subscriber   tongo.AccountID
		Admin        tongo.AccountID
		WithdrawTo   tongo.AccountID
	}

	JettonSwapAction struct {
		Dex        references.Dex
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
	DepositTokenStakeAction struct {
		Staker    tongo.AccountID
		Protocol  core.Protocol
		StakeMeta *core.Price
	}
	WithdrawTokenStakeRequestAction struct {
		Staker    tongo.AccountID
		Protocol  core.Protocol
		StakeMeta *core.Price
	}
	assetTransfer struct {
		Amount       big.Int
		IsTon        bool
		JettonMaster tongo.AccountID
		JettonWallet tongo.AccountID
	}

	PurchaseAction struct {
		Source, Destination tongo.AccountID
		InvoiceID           uuid.UUID
		Price               core.Price
	}

	AddExtensionAction struct {
		Wallet    tongo.AccountID
		Extension tongo.AccountID
	}

	RemoveExtensionAction struct {
		Wallet    tongo.AccountID
		Extension tongo.AccountID
	}

	SetSignatureAllowedAction struct {
		Wallet           tongo.AccountID
		SignatureAllowed bool
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
	case NftItemTransfer, ContractDeploy, UnSubscribe, JettonMint, JettonBurn, WithdrawStakeRequest, DomainRenew, ExtraCurrencyTransfer, DepositTokenStake, WithdrawTokenStakeRequest, AddExtension, RemoveExtension, SetSignatureAllowed: // actions without extra
		return 0
	case Purchase:
		if a.Purchase.Price.Currency.Type == core.CurrencyTON {
			return detectDirection(account, a.Purchase.Source, a.Purchase.Destination, a.Purchase.Price.Amount.Int64())
		}
		return 0
	case TonTransfer:
		return detectDirection(account, a.TonTransfer.Sender, a.TonTransfer.Recipient, a.TonTransfer.Amount)
	case SmartContractExec:
		return detectDirection(account, a.SmartContractExec.Executor, a.SmartContractExec.Contract, a.SmartContractExec.TonAttached)
	case GasRelay:
		if a.GasRelay.Relayer == account {
			return -a.GasRelay.Amount
		}
		return 0
	case NftPurchase:
		if a.NftPurchase.Price.Currency.Type == core.CurrencyTON {
			return detectDirection(account, a.NftPurchase.Buyer, a.NftPurchase.Seller, a.NftPurchase.Price.Amount.Int64())
		}
		return 0
	case AuctionBid:
		if a.AuctionBid.Amount.Currency.Type == core.CurrencyTON {
			return detectDirection(account, a.AuctionBid.Bidder, a.AuctionBid.Auction, a.AuctionBid.Amount.Amount.Int64())
		}
		return 0
	case ElectionsDepositStake:
		return detectDirection(account, a.ElectionsDepositStake.Staker, a.ElectionsDepositStake.Elector, a.ElectionsDepositStake.Amount)
	case ElectionsRecoverStake:
		return detectDirection(account, a.ElectionsRecoverStake.Elector, a.ElectionsRecoverStake.Staker, a.ElectionsRecoverStake.Amount)
	case Subscribe:
		if a.Subscribe.Price.Currency.Type == core.CurrencyTON {
			return detectDirection(account, a.Subscribe.Subscriber, a.Subscribe.WithdrawTo, a.Subscribe.Price.Amount.Int64())
		}
		return 0
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
		a.ExtraCurrencyTransfer,
		a.Subscribe,
		a.UnSubscribe,
		a.AuctionBid,
		a.ElectionsDepositStake,
		a.ElectionsRecoverStake,
		a.DepositStake,
		a.WithdrawStake,
		a.WithdrawStakeRequest,
		a.WithdrawTokenStakeRequest,
		a.DepositTokenStake,
		a.JettonSwap,
		a.JettonMint,
		a.JettonBurn,
		a.DnsRenew,
		a.Purchase,
		a.AddExtension,
		a.RemoveExtension,
		a.SetSignatureAllowed,
	} {
		if i != nil && !reflect.ValueOf(i).IsNil() {
			return slices.Contains(i.SubjectAccounts(), account)
		}
	}
	return false
}

func (a *AddExtensionAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Wallet}
}

func (a *RemoveExtensionAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Wallet}
}

func (a *SetSignatureAllowedAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Wallet}
}

func (a *TonTransferAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Sender, a.Recipient}
}

func (a *ExtraCurrencyTransferAction) SubjectAccounts() []tongo.AccountID {
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

func (a *SubscribeAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Subscriber, a.Admin}
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

func (a *UnSubscribeAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Subscriber, a.Admin}
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

func (a *DepositTokenStakeAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Staker}
}

func (a *WithdrawTokenStakeRequestAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Staker}
}

func (a *WithdrawStakeRequestAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Staker, a.Pool}
}

func (a *PurchaseAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Source, a.Destination}
}
