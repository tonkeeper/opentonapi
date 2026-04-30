package bath

import "github.com/tonkeeper/opentonapi/pkg/core"

const (
	AddExtension              = core.AddExtension
	AuctionBid                = core.AuctionBid
	ContractDeploy            = core.ContractDeploy
	DepositStake              = core.DepositStake
	DepositTokenStake         = core.DepositTokenStake
	DnsTgAuction              = core.DnsTgAuction
	DomainRenew               = core.DomainRenew
	ElectionsDepositStake     = core.ElectionsDepositStake
	ElectionsRecoverStake     = core.ElectionsRecoverStake
	ExtraCurrencyTransfer     = core.ExtraCurrencyTransfer
	FlawedJettonTransfer      = core.FlawedJettonTransfer
	GasRelay                  = core.GasRelay
	GetGemsAuction            = core.GetGemsAuction
	JettonBurn                = core.JettonBurn
	JettonMint                = core.JettonMint
	JettonSwap                = core.JettonSwap
	JettonTransfer            = core.JettonTransfer
	LiquidityDeposit          = core.LiquidityDeposit
	NftItemTransfer           = core.NftItemTransfer
	NftPurchase               = core.NftPurchase
	Purchase                  = core.Purchase
	RemoveExtension           = core.RemoveExtension
	SetSignatureAllowed       = core.SetSignatureAllowed
	SmartContractExec         = core.SmartContractExec
	Subscribe                 = core.Subscribe
	TonTransfer               = core.TonTransfer
	UnSubscribe               = core.UnSubscribe
	WithdrawStake             = core.WithdrawStake
	WithdrawStakeRequest      = core.WithdrawStakeRequest
	WithdrawTokenStakeRequest = core.WithdrawTokenStakeRequest
)

type (
	AccountValueFlow                = core.AccountValueFlow
	Action                          = core.Action
	ActionsList                     = core.ActionsList
	AddExtensionAction              = core.AddExtensionAction
	AuctionBidAction                = core.AuctionBidAction
	ContractDeployAction            = core.ContractDeployAction
	DepositStakeAction              = core.DepositStakeAction
	DepositTokenStakeAction         = core.DepositTokenStakeAction
	DnsRenewAction                  = core.DnsRenewAction
	ElectionsDepositStakeAction     = core.ElectionsDepositStakeAction
	ElectionsRecoverStakeAction     = core.ElectionsRecoverStakeAction
	EncryptedComment                = core.EncryptedComment
	ExtraCurrencyTransferAction     = core.ExtraCurrencyTransferAction
	FlawedJettonTransferAction      = core.FlawedJettonTransferAction
	GasRelayAction                  = core.GasRelayAction
	JettonBurnAction                = core.JettonBurnAction
	JettonMintAction                = core.JettonMintAction
	JettonSwapAction                = core.JettonSwapAction
	JettonTransferAction            = core.JettonTransferAction
	LiquidityDepositAction          = core.LiquidityDepositAction
	NftAuctionType                  = core.NftAuctionType
	NftPurchaseAction               = core.NftPurchaseAction
	NftTransferAction               = core.NftTransferAction
	PurchaseAction                  = core.PurchaseAction
	RemoveExtensionAction           = core.RemoveExtensionAction
	SetSignatureAllowedAction       = core.SetSignatureAllowedAction
	SmartContractAction             = core.SmartContractAction
	SubscribeAction                 = core.SubscribeAction
	TonTransferAction               = core.TonTransferAction
	UnSubscribeAction               = core.UnSubscribeAction
	ValueFlow                       = core.AccountsValueFlow
	WithdrawStakeAction             = core.WithdrawStakeAction
	WithdrawStakeRequestAction      = core.WithdrawStakeRequestAction
	WithdrawTokenStakeRequestAction = core.WithdrawTokenStakeRequestAction
	assetTransfer                   = core.AssetTransfer
)

func newValueFlow() *ValueFlow {
	return core.NewEmulatedValueFlow()
}
