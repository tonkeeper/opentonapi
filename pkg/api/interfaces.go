package api

import (
	"context"
	"crypto/ed25519"
	"time"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/liteclient"
	"github.com/tonkeeper/tongo/tep64"
	"github.com/tonkeeper/tongo/tlb"

	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/cache"
	"github.com/tonkeeper/opentonapi/pkg/core"
)

type storage interface {
	core.InformationSource

	// GetRawAccount returns low-level information about an account taken directly from the blockchain.
	GetRawAccount(ctx context.Context, id tongo.AccountID) (*core.Account, error)
	// GetRawAccounts returns low-level information about several accounts taken directly from the blockchain.
	GetRawAccounts(ctx context.Context, ids []tongo.AccountID) ([]*core.Account, error)
	// ReindexAccount updates internal cache used to store the account's state.
	ReindexAccount(ctx context.Context, accountID tongo.AccountID) error
	GetBlockHeader(ctx context.Context, id tongo.BlockID) (*core.BlockHeader, error)
	LastMasterchainBlockHeader(ctx context.Context) (*core.BlockHeader, error)
	GetTransaction(ctx context.Context, hash tongo.Bits256) (*core.Transaction, error)
	SearchTransactionByMessageHash(ctx context.Context, hash tongo.Bits256) (*tongo.Bits256, error)
	// GetBlockTransactions returns low-level information about transactions in a particular block.
	GetBlockTransactions(ctx context.Context, id tongo.BlockID) ([]*core.Transaction, error)
	GetAccountTransactions(ctx context.Context, id tongo.AccountID, limit int, beforeLt, afterLt uint64) ([]*core.Transaction, error)
	GetDnsExpiring(ctx context.Context, id tongo.AccountID, period *int) ([]core.DnsExpiring, error)

	GetTrace(ctx context.Context, hash tongo.Bits256) (*core.Trace, error)
	SearchTraces(ctx context.Context, a tongo.AccountID, limit int, beforeLT *int64, startTime *int64, endTime *int64) ([]tongo.Bits256, error)

	// GetStorageProviders returns a list of storage contracts deployed to the blockchain.
	GetStorageProviders(ctx context.Context) ([]core.StorageProvider, error)

	GetParticipatingInWhalesPools(ctx context.Context, id tongo.AccountID) ([]core.Nominator, error)
	GetWhalesPoolMemberInfo(ctx context.Context, pool, member tongo.AccountID) (core.Nominator, error)
	GetWhalesPoolInfo(ctx context.Context, id tongo.AccountID) (abi.GetParams_WhalesNominatorResult, abi.GetStakingStatusResult, int, error)
	GetParticipatingInTfPools(ctx context.Context, member tongo.AccountID) ([]core.Nominator, error)
	GetTFPools(ctx context.Context, onlyVerified bool) ([]core.TFPool, error)
	GetTFPool(ctx context.Context, pool tongo.AccountID) (core.TFPool, error)
	GetLiquidPool(ctx context.Context, pool tongo.AccountID) (core.LiquidPool, error)
	GetParticipatingInLiquidPools(ctx context.Context, member tongo.AccountID) ([]core.Nominator, error)
	GetLiquidPools(ctx context.Context, onlyVerified bool) ([]core.LiquidPool, error)

	GetNFTs(ctx context.Context, accounts []tongo.AccountID) ([]core.NftItem, error)
	SearchNFTs(ctx context.Context,
		collection *core.Filter[tongo.AccountID],
		owner *core.Filter[tongo.AccountID],
		includeOnSale bool,
		onlyVerified bool,
		limit, offset int,
	) ([]tongo.AccountID, error)
	GetNftCollections(ctx context.Context, limit, offset *int32) ([]core.NftCollection, error)
	GetNftCollectionByCollectionAddress(ctx context.Context, address tongo.AccountID) (core.NftCollection, error)

	FindAllDomainsResolvedToAddress(ctx context.Context, a tongo.AccountID, collections map[tongo.AccountID]string) ([]string, error)

	GetJettonWalletsByOwnerAddress(ctx context.Context, address tongo.AccountID) ([]core.JettonWallet, error)
	GetJettonMasterMetadata(ctx context.Context, master tongo.AccountID) (tongo.JettonMetadata, error)
	GetJettonMasterData(ctx context.Context, master tongo.AccountID) (abi.GetJettonDataResult, error)
	GetAccountJettonsHistory(ctx context.Context, address tongo.AccountID, limit int, beforeLT *int64, startTime *int64, endTime *int64) ([]tongo.Bits256, error)
	GetAccountJettonHistoryByID(ctx context.Context, address, jettonMaster tongo.AccountID, limit int, beforeLT *int64, startTime *int64, endTime *int64) ([]tongo.Bits256, error)

	GetAllAuctions(ctx context.Context) ([]core.Auction, error)
	GetDomainBids(ctx context.Context, domain string) ([]core.DomainBid, error)

	GetWalletPubKey(ctx context.Context, address tongo.AccountID) (ed25519.PublicKey, error)
	GetSubscriptions(ctx context.Context, address tongo.AccountID) ([]core.Subscription, error)
	GetJettonMasters(ctx context.Context, limit, offset int) ([]core.JettonMaster, error)

	GetLastConfig() (tlb.ConfigParams, error)

	GetSeqno(ctx context.Context, account tongo.AccountID) (uint32, error)

	liteStorageRaw
}

type liteStorageRaw interface {
	GetMasterchainInfoRaw(ctx context.Context) (liteclient.LiteServerMasterchainInfoC, error)
	GetMasterchainInfoExtRaw(ctx context.Context, mode uint32) (liteclient.LiteServerMasterchainInfoExtC, error)
	GetTimeRaw(ctx context.Context) (uint32, error)
	GetBlockRaw(ctx context.Context, id tongo.BlockIDExt) (liteclient.LiteServerBlockDataC, error)
	GetStateRaw(ctx context.Context, id tongo.BlockIDExt) (liteclient.LiteServerBlockStateC, error)
	GetBlockHeaderRaw(ctx context.Context, id tongo.BlockIDExt, mode uint32) (liteclient.LiteServerBlockHeaderC, error)
	SendMessageRaw(ctx context.Context, payload []byte) (uint32, error)
	GetAccountStateRaw(ctx context.Context, accountID tongo.AccountID) (liteclient.LiteServerAccountStateC, error)
	GetShardInfoRaw(ctx context.Context, id tongo.BlockIDExt, workchain uint32, shard uint64, exact bool) (liteclient.LiteServerShardInfoC, error)
	GetShardsAllInfo(ctx context.Context, id tongo.BlockIDExt) (liteclient.LiteServerAllShardsInfoC, error)
	GetTransactionsRaw(ctx context.Context, count uint32, accountID tongo.AccountID, lt uint64, hash tongo.Bits256) (liteclient.LiteServerTransactionListC, error)
	ListBlockTransactionsRaw(ctx context.Context, id tongo.BlockIDExt, mode, count uint32, after *liteclient.LiteServerTransactionId3C) (liteclient.LiteServerBlockTransactionsC, error)
	GetBlockProofRaw(ctx context.Context, knownBlock tongo.BlockIDExt, targetBlock *tongo.BlockIDExt) (liteclient.LiteServerPartialBlockProofC, error)
	GetConfigAllRaw(ctx context.Context, mode uint32, id tongo.BlockIDExt) (liteclient.LiteServerConfigInfoC, error)
	GetShardBlockProofRaw(ctx context.Context, id tongo.BlockIDExt) (liteclient.LiteServerShardBlockProofC, error)
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
	RunSmcMethodByID(context.Context, tongo.AccountID, int, tlb.VmStack) (uint32, tlb.VmStack, error)
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
	SearchAttachedAccountsByPrefix(prefix string) []addressbook.AttachedAccount
}

type ratesSource interface {
	GetRates(date time.Time) (map[string]float64, error)
}

type metadataCache struct {
	collectionsCache cache.Cache[tongo.AccountID, tep64.Metadata]
	jettonsCache     cache.Cache[tongo.AccountID, tep64.Metadata]
	storage          interface {
		GetJettonMasterMetadata(ctx context.Context, master tongo.AccountID) (tep64.Metadata, error)
		GetNftCollectionByCollectionAddress(ctx context.Context, address tongo.AccountID) (core.NftCollection, error)
	}
}

type mempoolEmulateCache struct {
	tracesCache         cache.Cache[tongo.Bits256, *core.Trace]
	accountsTracesCache cache.Cache[tongo.AccountID, []tongo.Bits256]
}
