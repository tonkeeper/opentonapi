package defi

import (
	"math/big"

	"github.com/tonkeeper/tongo/ton"
)

type AssetType string

const (
	AssetTypeStaking       AssetType = "staking"
	AssetTypeLiquidStaking AssetType = "liquid_staking"
	AssetTypeLiquidPool    AssetType = "liquid_pool"
	AssetTypeYieldToken    AssetType = "yield_token"
	AssetTypeLendingSupply AssetType = "lending_supply"
	AssetTypeLendingBorrow AssetType = "lending_borrow"
)

// Asset is a single DeFi position of an account
type Asset struct {
	Type     AssetType
	Provider Provider

	LockedAsset LockedAsset

	PoolAddress  *ton.AccountID
	AssetAddress *ton.AccountID
}

// LockedAsset is the underlying asset of a DeFi position
type LockedAsset struct {
	Type         LockedAssetType
	Amount       big.Int
	JettonMaster *ton.AccountID
}

type LockedAssetType string

const (
	LockedAssetTypeNative LockedAssetType = "native"
	LockedAssetTypeJetton LockedAssetType = "jetton"
)
