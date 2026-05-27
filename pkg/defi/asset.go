package defi

import "github.com/tonkeeper/tongo/ton"

// AssetType describes a DeFi position held by an account.
type AssetType string

const (
	AssetTypeStaking       AssetType = "staking"
	AssetTypeLiquidStaking AssetType = "liquid_staking"
	AssetTypeLiquidPool    AssetType = "liquid_pool"
	AssetTypeYieldToken    AssetType = "yield_token"
	AssetTypeLendingSupply AssetType = "lending_supply"
	AssetTypeLendingBorrow AssetType = "lending_borrow"
)

// LockedAssetType describes the underlying asset locked in a DeFi position.
type LockedAssetType string

const (
	LockedAssetTypeNative LockedAssetType = "native"
	LockedAssetTypeJetton LockedAssetType = "jetton"
)

// Asset is a single DeFi position of an account, independent of the API schema.
type Asset struct {
	Type         AssetType
	Amount       string
	PoolAddress  *ton.AccountID
	AssetAddress *ton.AccountID
	Provider     Provider
	LockedAsset  LockedAsset
}

// LockedAsset is the underlying asset of a DeFi position. For jetton assets it
// carries the master address; symbol, decimals and the rest are resolved from
// the jetton metadata.
type LockedAsset struct {
	Type         LockedAssetType
	JettonMaster *ton.AccountID
}
