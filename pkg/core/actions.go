package core

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"slices"

	"github.com/google/uuid"
	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

var zero = big.NewInt(0)

type ActionType string
type RefundType string
type NftAuctionType string

const (
	TonTransfer               ActionType = "TonTransfer"
	ExtraCurrencyTransfer     ActionType = "ExtraCurrencyTransfer"
	SmartContractExec         ActionType = "SmartContractExec"
	GasRelay                  ActionType = "GasRelay"
	NftItemTransfer           ActionType = "NftItemTransfer"
	NftPurchase               ActionType = "NftPurchase"
	JettonTransfer            ActionType = "JettonTransfer"
	FlawedJettonTransfer      ActionType = "FlawedJettonTransfer"
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
	LiquidityDeposit          ActionType = "LiquidityDeposit"
	UnknownAction             ActionType = "Unknown"
)

const (
	GetGemsAuction  NftAuctionType = "getgems"
	DnsTonAuction   NftAuctionType = "DNS.ton"
	DnsTgAuction    NftAuctionType = "DNS.tg"
	NumberTgAuction NftAuctionType = "NUMBER.tg"
)

type Action struct {
	TonTransfer               *TonTransferAction               `json:",omitempty"`
	ExtraCurrencyTransfer     *ExtraCurrencyTransferAction     `json:",omitempty"`
	SmartContractExec         *SmartContractAction             `json:",omitempty"`
	GasRelay                  *GasRelayAction                  `json:",omitempty"`
	NftItemTransfer           *NftTransferAction               `json:",omitempty"`
	NftPurchase               *NftPurchaseAction               `json:",omitempty"`
	JettonTransfer            *JettonTransferAction            `json:",omitempty"`
	FlawedJettonTransfer      *FlawedJettonTransferAction      `json:",omitempty"`
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
	LiquidityDepositAction    *LiquidityDepositAction          `json:",omitempty"`
	Success                   bool
	Type                      ActionType
	Error                     *string `json:",omitempty"`
	BaseTransactions          []ton.Bits256
}

type EncryptedComment struct {
	EncryptionType string
	CipherText     []byte
}

type AssetTransfer struct {
	Amount       big.Int
	IsTon        bool
	JettonMaster tongo.AccountID
	JettonWallet tongo.AccountID
}

type TonTransferAction struct {
	Amount           int64
	Comment          *string
	EncryptedComment *EncryptedComment
	Recipient        tongo.AccountID
	Sender           tongo.AccountID
	Refund           *Refund
}

type ExtraCurrencyTransferAction struct {
	CurrencyID       int32
	Amount           tlb.VarUInteger32
	Comment          *string
	EncryptedComment *EncryptedComment
	Recipient        tongo.AccountID
	Sender           tongo.AccountID
}

type SmartContractAction struct {
	TonAttached int64
	Executor    tongo.AccountID
	Contract    tongo.AccountID
	Operation   string
	Payload     string
}

type GasRelayAction struct {
	Amount  int64
	Relayer tongo.AccountID
	Target  tongo.AccountID
}

type NftTransferAction struct {
	Comment          *string
	EncryptedComment *EncryptedComment
	Recipient        *tongo.AccountID
	Sender           *tongo.AccountID
	Nft              tongo.AccountID
	Refund           *Refund
}

type NftPurchaseAction struct {
	Nft         tongo.AccountID
	Buyer       tongo.AccountID
	Seller      tongo.AccountID
	AuctionType NftAuctionType
	Price       Price
}

type ElectionsDepositStakeAction struct {
	Amount  int64
	Elector tongo.AccountID
	Staker  tongo.AccountID
}

type ElectionsRecoverStakeAction struct {
	Amount  int64
	Elector tongo.AccountID
	Staker  tongo.AccountID
}

type JettonTransferAction struct {
	Comment          *string
	EncryptedComment *EncryptedComment
	Jetton           tongo.AccountID
	Recipient        *tongo.AccountID
	Sender           *tongo.AccountID
	RecipientsWallet tongo.AccountID
	SendersWallet    tongo.AccountID
	Amount           tlb.VarUInteger16
	Refund           *Refund
	IsWrappedTon     bool `json:"-"`
}

type FlawedJettonTransferAction struct {
	Comment          *string
	EncryptedComment *EncryptedComment
	Jetton           tongo.AccountID
	Recipient        *tongo.AccountID
	Sender           *tongo.AccountID
	RecipientsWallet tongo.AccountID
	SendersWallet    tongo.AccountID
	SentAmount       tlb.VarUInteger16
	ReceivedAmount   tlb.VarUInteger16
	Refund           *Refund
	IsWrappedTon     bool `json:"-"`
}

type JettonMintAction struct {
	Jetton           tongo.AccountID
	Recipient        tongo.AccountID
	RecipientsWallet tongo.AccountID
	Amount           tlb.VarUInteger16
}

type JettonBurnAction struct {
	Jetton        tongo.AccountID
	Sender        tongo.AccountID
	SendersWallet tongo.AccountID
	Amount        tlb.VarUInteger16
}

type ContractDeployAction struct {
	Address    tongo.AccountID
	Interfaces []abi.ContractInterface
}

type SubscribeAction struct {
	Subscription tongo.AccountID
	Subscriber   tongo.AccountID
	Admin        tongo.AccountID
	WithdrawTo   tongo.AccountID
	Price        Price
	First        bool
}

type UnSubscribeAction struct {
	Subscription tongo.AccountID
	Subscriber   tongo.AccountID
	Admin        tongo.AccountID
	WithdrawTo   tongo.AccountID
}

type JettonSwapAction struct {
	Dex        references.Dex
	UserWallet tongo.AccountID
	Router     tongo.AccountID
	In         AssetTransfer
	Out        AssetTransfer
}

type DepositStakeAction struct {
	Staker         tongo.AccountID
	Amount         int64
	Pool           tongo.AccountID
	Implementation StakingImplementation
}

type WithdrawStakeAction struct {
	Staker         tongo.AccountID
	Amount         int64
	Pool           tongo.AccountID
	Implementation StakingImplementation
}

type WithdrawStakeRequestAction struct {
	Staker         tongo.AccountID
	Amount         *int64
	Pool           tongo.AccountID
	Implementation StakingImplementation
}

type DepositTokenStakeAction struct {
	Staker    tongo.AccountID
	Protocol  Protocol
	StakeMeta *Price
}

type WithdrawTokenStakeRequestAction struct {
	Staker    tongo.AccountID
	Protocol  Protocol
	StakeMeta *Price
}

type AuctionBidAction struct {
	Type       NftAuctionType
	Amount     Price
	Nft        *NftItem
	NftAddress *tongo.AccountID
	Bidder     tongo.AccountID
	Auction    tongo.AccountID
}

type DnsRenewAction struct {
	Item    tongo.AccountID
	Renewer tongo.AccountID
}

type PurchaseAction struct {
	Source, Destination tongo.AccountID
	InvoiceID           uuid.UUID
	Price               Price
}

type AddExtensionAction struct {
	Wallet    tongo.AccountID
	Extension tongo.AccountID
}

type RemoveExtensionAction struct {
	Wallet    tongo.AccountID
	Extension tongo.AccountID
}

type SetSignatureAllowedAction struct {
	Wallet           tongo.AccountID
	SignatureAllowed bool
}

type LiquidityDepositAction struct {
	Protocol Protocol
	From     tongo.AccountID
	Tokens   []VaultDepositInfo
}

type Refund struct {
	Type   RefundType
	Origin string
}

func NewEmulatedValueFlow() *AccountsValueFlow {
	return &AccountsValueFlow{
		Accounts: map[tongo.AccountID]*AccountValueFlow{},
	}
}

func (a Action) String() string {
	val := reflect.ValueOf(a)
	f := val.FieldByName(string(a.Type))
	b, _ := json.Marshal(f.Interface())
	return fmt.Sprintf("%v: %+v", a.Type, string(b))
}

func (a Action) ContributeToExtra(accountID tongo.AccountID) int64 {
	if !a.Success {
		return 0
	}
	switch a.Type {
	case NftItemTransfer, ContractDeploy, UnSubscribe, JettonMint, JettonBurn, WithdrawStakeRequest, DomainRenew, ExtraCurrencyTransfer, DepositTokenStake, WithdrawTokenStakeRequest, AddExtension, RemoveExtension, SetSignatureAllowed, FlawedJettonTransfer:
		return 0
	case Purchase:
		if a.Purchase.Price.Currency.Type == CurrencyTON {
			return detectDirection(accountID, a.Purchase.Source, a.Purchase.Destination, a.Purchase.Price.Amount.Int64())
		}
		return 0
	case TonTransfer:
		return detectDirection(accountID, a.TonTransfer.Sender, a.TonTransfer.Recipient, a.TonTransfer.Amount)
	case SmartContractExec:
		return detectDirection(accountID, a.SmartContractExec.Executor, a.SmartContractExec.Contract, a.SmartContractExec.TonAttached)
	case GasRelay:
		if a.GasRelay.Relayer == accountID {
			return -a.GasRelay.Amount
		}
		return 0
	case NftPurchase:
		if a.NftPurchase.Price.Currency.Type == CurrencyTON {
			return detectDirection(accountID, a.NftPurchase.Buyer, a.NftPurchase.Seller, a.NftPurchase.Price.Amount.Int64())
		}
		return 0
	case AuctionBid:
		if a.AuctionBid.Amount.Currency.Type == CurrencyTON {
			return detectDirection(accountID, a.AuctionBid.Bidder, a.AuctionBid.Auction, a.AuctionBid.Amount.Amount.Int64())
		}
		return 0
	case ElectionsDepositStake:
		return detectDirection(accountID, a.ElectionsDepositStake.Staker, a.ElectionsDepositStake.Elector, a.ElectionsDepositStake.Amount)
	case ElectionsRecoverStake:
		return detectDirection(accountID, a.ElectionsRecoverStake.Elector, a.ElectionsRecoverStake.Staker, a.ElectionsRecoverStake.Amount)
	case Subscribe:
		if a.Subscribe.Price.Currency.Type == CurrencyTON {
			return detectDirection(accountID, a.Subscribe.Subscriber, a.Subscribe.WithdrawTo, a.Subscribe.Price.Amount.Int64())
		}
		return 0
	case DepositStake:
		return detectDirection(accountID, a.DepositStake.Staker, a.DepositStake.Pool, a.DepositStake.Amount)
	case JettonTransfer:
		if !a.JettonTransfer.IsWrappedTon {
			return 0
		}
		if a.JettonTransfer.Sender == nil || a.JettonTransfer.Recipient == nil {
			return 0
		}
		b := big.Int(a.JettonTransfer.Amount)
		return detectDirection(accountID, *a.JettonTransfer.Sender, *a.JettonTransfer.Recipient, b.Int64())
	case JettonSwap:
		if accountID != a.JettonSwap.UserWallet {
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
		return detectDirection(accountID, a.WithdrawStake.Pool, a.WithdrawStake.Staker, a.WithdrawStake.Amount)
	case LiquidityDeposit:
		extra := int64(0)
		for _, token := range a.LiquidityDepositAction.Tokens {
			if accountID == a.LiquidityDepositAction.From && token.Price.Currency.Type == CurrencyTON {
				extra -= token.Price.Amount.Int64()
			}
			if accountID == token.Vault && token.Price.Currency.Type == CurrencyTON {
				extra += token.Price.Amount.Int64()
			}
		}
		return extra
	default:
		panic("unknown action type")
	}
}

func (a Action) IsSubject(accountID tongo.AccountID) bool {
	for _, i := range []interface {
		SubjectAccounts() []tongo.AccountID
	}{
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
		a.LiquidityDepositAction,
	} {
		if i != nil && !reflect.ValueOf(i).IsNil() {
			return slices.Contains(i.SubjectAccounts(), accountID)
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

func (a *FlawedJettonTransferAction) SubjectAccounts() []tongo.AccountID {
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

func (a *DnsRenewAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Renewer, a.Item}
}

func (a *GasRelayAction) SubjectAccounts() []tongo.AccountID {
	return []tongo.AccountID{a.Relayer}
}

func (a *LiquidityDepositAction) SubjectAccounts() []tongo.AccountID {
	accounts := make([]tongo.AccountID, 0, 3)
	accounts = append(accounts, a.From)
	for _, token := range a.Tokens {
		accounts = append(accounts, token.Vault)
	}
	return accounts
}

func (a *JettonTransferAction) PayloadFromABI(payload abi.JettonPayload) {
	switch payload.SumType {
	case abi.TextCommentJettonOp:
		a.Comment = g.Pointer(string(payload.Value.(abi.TextCommentJettonPayload).Text))
	case abi.EncryptedTextCommentJettonOp:
		a.EncryptedComment = &EncryptedComment{
			CipherText:     payload.Value.(abi.EncryptedTextCommentJettonPayload).CipherText,
			EncryptionType: "simple",
		}
	case abi.EmptyJettonOp:
	default:
		if payload.SumType != abi.UnknownJettonOp {
			a.Comment = g.Pointer("Call: " + payload.SumType)
		} else if payload.OpCode != nil {
			a.Comment = g.Pointer(fmt.Sprintf("Call: 0x%08x", *payload.OpCode))
		}
	}
}

func (a *FlawedJettonTransferAction) PayloadFromABI(payload abi.JettonPayload) {
	switch payload.SumType {
	case abi.TextCommentJettonOp:
		a.Comment = g.Pointer(string(payload.Value.(abi.TextCommentJettonPayload).Text))
	case abi.EncryptedTextCommentJettonOp:
		a.EncryptedComment = &EncryptedComment{
			CipherText:     payload.Value.(abi.EncryptedTextCommentJettonPayload).CipherText,
			EncryptionType: "simple",
		}
	case abi.EmptyJettonOp:
	default:
		if payload.SumType != abi.UnknownJettonOp {
			a.Comment = g.Pointer("Call: " + payload.SumType)
		} else if payload.OpCode != nil {
			a.Comment = g.Pointer(fmt.Sprintf("Call: 0x%08x", *payload.OpCode))
		}
	}
}

func (msg *ExtInMsgCopy) IsEmulation() bool {
	return msg.Accounts != nil
}

func (al *ActionsList) Extra(accountID tongo.AccountID) int64 {
	extra := int64(0)
	if flow, ok := al.ValueFlow.Accounts[accountID]; ok {
		extra += flow.Ton
	}
	for _, action := range al.Actions {
		extra -= action.ContributeToExtra(accountID)
	}
	return extra
}

func (fl *AccountsValueFlow) AddTons(accountID tongo.AccountID, amount int64) {
	if accountFlow, ok := fl.Accounts[accountID]; ok {
		accountFlow.Ton += amount
		return
	}
	fl.Accounts[accountID] = &AccountValueFlow{Ton: amount}
}

func (fl *AccountsValueFlow) AddFee(accountID tongo.AccountID, amount int64) {
	if accountFlow, ok := fl.Accounts[accountID]; ok {
		accountFlow.Fees += amount
		accountFlow.Ton -= amount
		return
	}
	fl.Accounts[accountID] = &AccountValueFlow{Fees: amount, Ton: -amount}
}
func (fl *AccountsValueFlow) SubJettons(accountID tongo.AccountID, jettonMaster tongo.AccountID, value big.Int) {
	var negative big.Int
	negative.Neg(&value)
	fl.AddJettons(accountID, jettonMaster, negative)
}

func (fl *AccountsValueFlow) AddJettons(accountID tongo.AccountID, jettonMaster tongo.AccountID, value big.Int) {
	accountFlow, ok := fl.Accounts[accountID]
	if !ok {
		accountFlow = &AccountValueFlow{}
		fl.Accounts[accountID] = accountFlow
	}
	if accountFlow.Jettons == nil {
		accountFlow.Jettons = make(map[tongo.AccountID]big.Int, 1)
	}
	current := accountFlow.Jettons[jettonMaster]
	newValue := big.Int{}
	newValue.Add(&current, &value)
	accountFlow.Jettons[jettonMaster] = newValue
	if newValue.Cmp(zero) == 0 {
		delete(accountFlow.Jettons, jettonMaster)
	}
}

func (fl *AccountsValueFlow) Merge(other *AccountsValueFlow) {
	for accountID, af := range other.Accounts {
		if _, ok := fl.Accounts[accountID]; !ok {
			fl.Accounts[accountID] = &AccountValueFlow{}
		}
		fl.Accounts[accountID].Ton += af.Ton
		fl.Accounts[accountID].Fees += af.Fees
		fl.Accounts[accountID].NFTs[0] += af.NFTs[0]
		fl.Accounts[accountID].NFTs[1] += af.NFTs[1]
		for jetton, value := range af.Jettons {
			fl.AddJettons(accountID, jetton, value)
		}
	}
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
