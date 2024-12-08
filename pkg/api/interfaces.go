package api

import (
	"context"
	"crypto/ed25519"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/gasless"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/liteclient"
	"github.com/tonkeeper/tongo/tep64"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"

	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/blockchain"
	"github.com/tonkeeper/opentonapi/pkg/cache"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/rates"
)

type storage interface {
	core.InformationSource

	// GetRawAccount returns low-level information about an account taken directly from the blockchain.
	GetRawAccount(ctx context.Context, id tongo.AccountID) (*core.Account, error)
	GetContract(ctx context.Context, id tongo.AccountID) (*core.Contract, error)
	// GetRawAccounts returns low-level information about several accounts taken directly from the blockchain.
	GetRawAccounts(ctx context.Context, ids []tongo.AccountID) ([]*core.Account, error)
	// ReindexAccount updates internal cache used to store the account's state.
	ReindexAccount(ctx context.Context, accountID tongo.AccountID) error
	GetBlockHeader(ctx context.Context, id tongo.BlockID) (*core.BlockHeader, error)
	GetReducedBlocks(ctx context.Context, from, to int64) ([]core.ReducedBlock, error)
	GetBlockShards(ctx context.Context, id tongo.BlockID) ([]ton.BlockID, error)
	LastMasterchainBlockHeader(ctx context.Context) (*core.BlockHeader, error)
	GetTransaction(ctx context.Context, hash tongo.Bits256) (*core.Transaction, error)
	SearchTransactionByMessageHash(ctx context.Context, hash tongo.Bits256) (*tongo.Bits256, error)
	// GetBlockTransactions returns low-level information about transactions in a particular block.
	GetBlockTransactions(ctx context.Context, id tongo.BlockID) ([]*core.Transaction, error)
	GetAccountTransactions(ctx context.Context, id tongo.AccountID, limit int, beforeLt, afterLt uint64, descendingOrder bool) ([]*core.Transaction, error)
	GetDnsExpiring(ctx context.Context, id tongo.AccountID, period *int) ([]core.DnsExpiring, error)
	GetLogs(ctx context.Context, account tongo.AccountID, destination *tlb.MsgAddress, limit int, beforeLT uint64) ([]core.Message, error)
	GetAccountDiff(ctx context.Context, account tongo.AccountID, startTime int64, endTime int64) (int64, error)
	GetLatencyAndLastMasterchainSeqno(ctx context.Context) (int64, uint32, error)
	GetTrace(ctx context.Context, hash tongo.Bits256) (*core.Trace, error)
	SearchTraces(ctx context.Context, a tongo.AccountID, limit int, beforeLT, startTime, endTime *int64, initiator bool) ([]core.TraceID, error)

	// GetStorageProviders returns a list of storage contracts deployed to the blockchain.
	GetStorageProviders(ctx context.Context) ([]core.StorageProvider, error)

	GetParticipatingInWhalesPools(ctx context.Context, id tongo.AccountID) ([]core.Nominator, error)
	GetWhalesPoolMemberInfo(ctx context.Context, pool, member tongo.AccountID) (core.Nominator, error)
	GetWhalesPoolInfo(ctx context.Context, id tongo.AccountID) (abi.GetParams_WhalesNominatorResult, abi.GetStakingStatusResult, int, uint64, error)
	GetParticipatingInTfPools(ctx context.Context, member tongo.AccountID) ([]core.Nominator, error)
	GetTFPools(ctx context.Context, onlyVerified bool, availableFor *tongo.AccountID) ([]core.TFPool, error)
	GetTFPool(ctx context.Context, pool tongo.AccountID) (core.TFPool, error)
	GetLiquidPool(ctx context.Context, pool tongo.AccountID) (core.LiquidPool, error)
	GetParticipatingInLiquidPools(ctx context.Context, member tongo.AccountID) ([]core.Nominator, error)
	GetLiquidPools(ctx context.Context, onlyVerified bool) ([]core.LiquidPool, error)

	GetNFTs(ctx context.Context, accounts []tongo.AccountID) ([]core.NftItem, error)
	SearchNFTs(ctx context.Context, collection *core.Filter[tongo.AccountID], owner *core.Filter[tongo.AccountID], includeOnSale bool, onlyVerified bool, limit, offset int) ([]tongo.AccountID, error)
	GetNftCollections(ctx context.Context, limit, offset *int32) ([]core.NftCollection, error)
	GetNftCollectionsByAddresses(ctx context.Context, addresses []ton.AccountID) ([]core.NftCollection, error)
	GetNftCollectionByCollectionAddress(ctx context.Context, address tongo.AccountID) (core.NftCollection, error)
	GetAccountNftsHistory(ctx context.Context, address tongo.AccountID, limit int, beforeLT *int64, startTime *int64, endTime *int64) ([]tongo.Bits256, error)
	GetNftHistory(ctx context.Context, address tongo.AccountID, limit int, beforeLT *int64, startTime *int64, endTime *int64) ([]tongo.Bits256, error)

	FindAllDomainsResolvedToAddress(ctx context.Context, a tongo.AccountID, collections map[tongo.AccountID]string) ([]string, error)

	GetJettonWalletsByOwnerAddress(ctx context.Context, address tongo.AccountID, jetton *tongo.AccountID, isJettonMaster bool, mintless bool) ([]core.JettonWallet, error)
	GetJettonsHoldersCount(ctx context.Context, accounts []tongo.AccountID) (map[tongo.AccountID]int32, error)
	GetJettonHolders(ctx context.Context, jettonMaster tongo.AccountID, limit, offset int) ([]core.JettonHolder, error)
	GetJettonMasterMetadata(ctx context.Context, master tongo.AccountID) (tongo.JettonMetadata, error)
	GetJettonMasterData(ctx context.Context, master tongo.AccountID) (core.JettonMaster, error)
	GetAccountJettonsHistory(ctx context.Context, address tongo.AccountID, limit int, beforeLT, startTime, endTime *int64) ([]tongo.Bits256, error)
	GetAccountJettonHistoryByID(ctx context.Context, address, jettonMaster tongo.AccountID, limit int, beforeLT, startTime, endTime *int64) ([]tongo.Bits256, error)
	GetJettonTransferPayload(ctx context.Context, accountID, jettonMaster ton.AccountID) (*core.JettonTransferPayload, error)

	GetAllAuctions(ctx context.Context) ([]core.Auction, error)
	GetDomainBids(ctx context.Context, domain string) ([]core.DomainBid, error)
	GetDomainInfo(ctx context.Context, domain string) (core.NftItem, int64, error)

	GetWalletPubKey(ctx context.Context, address tongo.AccountID) (ed25519.PublicKey, error)
	GetSubscriptions(ctx context.Context, address tongo.AccountID) ([]core.Subscription, error)
	GetJettonMasters(ctx context.Context, limit, offset int) ([]core.JettonMaster, error)
	GetJettonMastersByAddresses(ctx context.Context, addresses []ton.AccountID) ([]core.JettonMaster, error)

	GetLastConfig(ctx context.Context) (ton.BlockchainConfig, error)
	GetConfigRaw(ctx context.Context) ([]byte, error)
	GetConfigFromBlock(ctx context.Context, id ton.BlockID) (tlb.ConfigParams, error)

	GetSeqno(ctx context.Context, account tongo.AccountID) (uint32, error)

	GetAccountState(ctx context.Context, a tongo.AccountID) (tlb.ShardAccount, error)
	GetLibraries(ctx context.Context, libraries []tongo.Bits256) (map[tongo.Bits256]*boc.Cell, error)

	SearchAccountsByPubKey(ctx context.Context, pubKey ed25519.PublicKey) ([]tongo.AccountID, error)

	// TrimmedConfigBase64 returns the current trimmed blockchain config in a base64 format.
	TrimmedConfigBase64() (string, error)

	GetInscriptionBalancesByAccount(ctx context.Context, a ton.AccountID) ([]core.InscriptionBalance, error)
	GetInscriptionsHistoryByAccount(ctx context.Context, a ton.AccountID, ticker *string, beforeLt int64, limit int) ([]core.InscriptionMessage, error)

	GetMissedEvents(ctx context.Context, account ton.AccountID, lt uint64, limit int) ([]oas.AccountEvent, error)

	GetAccountMultisigs(ctx context.Context, accountID ton.AccountID) ([]core.Multisig, error)
	GetMultisigByID(ctx context.Context, accountID ton.AccountID) (*core.Multisig, error)

	SaveTraceWithState(ctx context.Context, msgHash string, trace *core.Trace, getMethods []abi.MethodInvocation, ttl time.Duration) error
	GetTraceWithState(ctx context.Context, msgHash string) (*core.Trace, []abi.MethodInvocation, error)

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
	GetAccountStateRaw(ctx context.Context, accountID tongo.AccountID, id *tongo.BlockIDExt) (liteclient.LiteServerAccountStateC, error)
	GetShardInfoRaw(ctx context.Context, id tongo.BlockIDExt, workchain uint32, shard uint64, exact bool) (liteclient.LiteServerShardInfoC, error)
	GetShardsAllInfo(ctx context.Context, id tongo.BlockIDExt) (liteclient.LiteServerAllShardsInfoC, error)
	GetTransactionsRaw(ctx context.Context, count uint32, accountID tongo.AccountID, lt uint64, hash tongo.Bits256) (liteclient.LiteServerTransactionListC, error)
	ListBlockTransactionsRaw(ctx context.Context, id tongo.BlockIDExt, mode, count uint32, after *liteclient.LiteServerTransactionId3C) (liteclient.LiteServerBlockTransactionsC, error)
	GetBlockProofRaw(ctx context.Context, knownBlock tongo.BlockIDExt, targetBlock *tongo.BlockIDExt) (liteclient.LiteServerPartialBlockProofC, error)
	GetConfigAllRaw(ctx context.Context, mode uint32, id tongo.BlockIDExt) (liteclient.LiteServerConfigInfoC, error)
	GetShardBlockProofRaw(ctx context.Context, id tongo.BlockIDExt) (liteclient.LiteServerShardBlockProofC, error)
	GetOutMsgQueueSizes(ctx context.Context) (liteclient.LiteServerOutMsgQueueSizesC, error)
}

// chainState provides current blockchain state which change very rarely or slow like staking APY income
type chainState interface {
	GetAPY() float64
	CheckIsSuspended(id tongo.AccountID) bool
}

// messageSender provides a method to send a raw message to the blockchain.
type messageSender interface {
	// SendMessage sends the given message to the blockchain.
	SendMessage(ctx context.Context, msgCopy blockchain.ExtInMsgCopy) error
	// SendMultipleMessages sends a list of messages to the cache for later sending.
	SendMultipleMessages(ctx context.Context, copies []blockchain.ExtInMsgCopy)
}

// executor runs any get methods
type executor interface {
	RunSmcMethod(context.Context, tongo.AccountID, string, tlb.VmStack) (uint32, tlb.VmStack, error)
	RunSmcMethodByID(context.Context, tongo.AccountID, int, tlb.VmStack) (uint32, tlb.VmStack, error)
}

// addressBook provides methods to request additional information about accounts, NFT collections and jettons
// The information is stored in "https://github.com/tonkeeper/ton-assets/" and
// is being maintained by the tonkeeper team and the community.
type addressBook interface {
	IsWallet(a tongo.AccountID) (bool, error)
	GetAddressInfoByAddress(a tongo.AccountID) (addressbook.KnownAddress, bool) // todo: maybe rewrite to pointer
	GetCollectionInfoByAddress(a tongo.AccountID) (addressbook.KnownCollection, bool)
	GetJettonInfoByAddress(a tongo.AccountID) (addressbook.KnownJetton, bool)
	GetTFPoolInfo(a tongo.AccountID) (addressbook.TFPoolInfo, bool)
	GetKnownJettons() map[tongo.AccountID]addressbook.KnownJetton
	GetKnownCollections() map[tongo.AccountID]addressbook.KnownCollection
	SearchAttachedAccountsByPrefix(prefix string) []addressbook.AttachedAccount
}

type Gasless interface {
	Config(ctx context.Context) (gasless.Config, error)
	Estimate(ctx context.Context, masterID ton.AccountID, walletAddress ton.AccountID, walletPubkey []byte, messages []string) (gasless.SignRawParams, error)
	Send(ctx context.Context, walletPublicKey ed25519.PublicKey, payload []byte) error
}

type ratesSource interface {
	GetRates(date int64) (map[string]float64, error)
	GetRatesChart(token string, currency string, pointsCount int, startDate *int64, endDate *int64) ([]rates.Point, error)
	GetMarketsTonPrice() ([]rates.Market, error)
}

type SpamFilter interface {
	CheckActions(actions []oas.Action, viewer *ton.AccountID, initiator ton.AccountID) bool
	JettonTrust(address tongo.AccountID, symbol, name, image string) core.TrustType
	NftTrust(address tongo.AccountID, collection *ton.AccountID, description, image string) core.TrustType
}

type metadataCache struct {
	collectionsCache cache.Cache[tongo.AccountID, tep64.Metadata]
	jettonsCache     cache.Cache[tongo.AccountID, tep64.Metadata]
	storage          interface {
		GetJettonMasterMetadata(ctx context.Context, master tongo.AccountID) (tep64.Metadata, error)
		GetNftCollectionByCollectionAddress(ctx context.Context, address tongo.AccountID) (core.NftCollection, error)
	}
}

type mempoolEmulate struct {
	traces         cache.Cache[ton.Bits256, *core.Trace]
	accountsTraces cache.Cache[tongo.AccountID, []ton.Bits256]
}
