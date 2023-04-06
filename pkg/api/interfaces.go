package api

import (
	"context"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"

	"github.com/tonkeeper/opentonapi/pkg/core"
)

type storage interface {
	// GetAccountInfo returns human-friendly information about an account without low-level details.
	GetAccountInfo(ctx context.Context, id tongo.AccountID) (*core.AccountInfo, error)
	// GetRawAccount returns low-level information about an account taken directly from the blockchain.
	GetRawAccount(ctx context.Context, id tongo.AccountID) (*core.Account, error)
	GetBlockHeader(ctx context.Context, id tongo.BlockID) (*core.BlockHeader, error)
	LastMasterchainBlockHeader(ctx context.Context) (*core.BlockHeader, error)
	GetTransaction(ctx context.Context, hash tongo.Bits256) (*core.Transaction, error)
	// GetBlockTransactions returns low-level information about transactions in a particular block.
	GetBlockTransactions(ctx context.Context, id tongo.BlockID) ([]*core.Transaction, error)
	GetAccountTransactions(ctx context.Context, id tongo.AccountID, limit int, beforeLt, afterLt uint64) ([]*core.Transaction, error)

	GetTrace(ctx context.Context, hash tongo.Bits256) (*core.Trace, error)
	SearchTraces(ctx context.Context, a tongo.AccountID, limit int, beforeLT *int64, startTime *int64, endTime *int64) ([]tongo.Bits256, error)

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
	GetJettonMasterMetadata(ctx context.Context, master tongo.AccountID) (tongo.JettonMetadata, error)
	GetJettonMasterData(ctx context.Context, master tongo.AccountID) (abi.GetJettonDataResult, error)
}

// chainState provides current blockchain state which change very rarely or slow like staking APY income
type chainState interface {
	GetAPY() float64
}

// messageSender provides a method to send a raw message to the blockchain.
type messageSender interface {
	// SendMessage sends the given payload(a message) to the blockchain.
	SendMessage(ctx context.Context, payload []byte) error
}

// executor runs any get methods
type executor interface {
	RunSmcMethod(context.Context, tongo.AccountID, string, tlb.VmStack) (uint32, tlb.VmStack, error)
}

type previewGenerator interface {
	GenerateImageUrl(url string, height, width int) string
}

// addressBook provides methods to request additional information about accounts, NFT collections and jettons
// The information is stored in "https://github.com/tonkeeper/ton-assets/" and
// is being maintained by the tonkeeper team and the community.
type addressBook interface {
	GetAddressInfoByAddress(a tongo.AccountID) (addressbook.KnownAddress, bool)
	GetCollectionInfoByAddress(a tongo.AccountID) (addressbook.KnownCollection, bool)
	GetJettonInfoByAddress(a tongo.AccountID) (addressbook.KnownJetton, bool)
	GetTFPoolInfo(a tongo.AccountID) (addressbook.TFPoolInfo, bool)
	GetKnownJettons() map[tongo.AccountID]addressbook.KnownJetton
}
