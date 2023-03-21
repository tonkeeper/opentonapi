package api

import (
	"context"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"

	"github.com/tonkeeper/opentonapi/pkg/core"
)

type storage interface {
	// GetAccountInfo returns human-friendly information about an account without low-level details.
	GetAccountInfo(ctx context.Context, id tongo.AccountID) (*core.AccountInfo, error)
	// GetRawAccount returns low-level information about an account taken directly from the blockchain.
	GetRawAccount(ctx context.Context, id tongo.AccountID) (*core.Account, error)
	GetBlockHeader(ctx context.Context, id tongo.BlockID) (*core.BlockHeader, error)
	GetTransaction(ctx context.Context, hash tongo.Bits256) (*core.Transaction, error)
	// GetBlockTransactions returns low-level information about transactions in a particular block.
	GetBlockTransactions(ctx context.Context, id tongo.BlockID) ([]*core.Transaction, error)
	GetTrace(ctx context.Context, hash tongo.Bits256) (*core.Trace, error)
	// GetStorageProviders returns a list of storage contracts deployed to the blockchain.
	GetStorageProviders(ctx context.Context) ([]core.StorageProvider, error)

	GetParticipatingInWhalesPools(ctx context.Context, id tongo.AccountID) ([]core.WhalesNominator, error)
	GetWhalesPoolMemberInfo(ctx context.Context, pool, member tongo.AccountID) (core.WhalesNominator, error)
	GetWhalesPoolInfo(ctx context.Context, id tongo.AccountID) (abi.GetParams_WhalesNominatorResult, abi.GetStakingStatusResult, error)
	GetTFPools(ctx context.Context) ([]core.TFPool, error)
	GetTFPool(ctx context.Context, pool tongo.AccountID) (core.TFPool, error)

	GetNFTs(ctx context.Context, accounts []tongo.AccountID) ([]core.NftItem, error)
	SearchNFTs(ctx context.Context,
		collection *core.Filter[tongo.AccountID],
		owner *core.Filter[tongo.AccountID],
		includeOnSale bool,
		onlyVerified bool,
		limit, offset int,
	) ([]tongo.AccountID, error)

	GetJettonWalletsByOwnerAddress(ctx context.Context, address tongo.AccountID) ([]core.JettonWallet, error)
	GetJettonMasterMetadata(ctx context.Context, master tongo.AccountID) (core.JettonMetadata, error)
}

type chainState interface {
	GetAPY() float64
}
