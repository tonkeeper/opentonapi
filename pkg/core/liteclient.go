package core

import (
	"context"

	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/liteclient"
	"github.com/tonkeeper/tongo/tep64"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

// LiteClient is the blockchain connection everything here reads through:
// litestorage, the elector iterators, the rewards service. It is deliberately
// an interface rather than tongo's *liteapi.Client, so that a deployment can
// supply a different implementation - one talking to its own pool of
// lightservers, say - without that implementation having to live in this
// repository.
//
// The method set is tongo's, so *liteapi.Client is one line away from
// satisfying it; see LiteAPIClient for the line in question.
type LiteClient interface {
	// Masterchain and blocks.
	GetMasterchainInfo(ctx context.Context) (liteclient.LiteServerMasterchainInfoC, error)
	GetMasterchainInfoExt(ctx context.Context, mode uint32) (liteclient.LiteServerMasterchainInfoExtC, error)
	GetTime(ctx context.Context) (uint32, error)
	LookupBlock(ctx context.Context, blockID ton.BlockID, mode uint32, lt *uint64, utime *uint32) (ton.BlockIDExt, tlb.BlockInfo, error)
	GetBlock(ctx context.Context, blockID ton.BlockIDExt) (tlb.Block, error)
	GetBlockRaw(ctx context.Context, blockID ton.BlockIDExt) (liteclient.LiteServerBlockDataC, error)
	GetBlockHeaderRaw(ctx context.Context, blockID ton.BlockIDExt, mode uint32) (liteclient.LiteServerBlockHeaderC, error)
	GetStateRaw(ctx context.Context, blockID ton.BlockIDExt) (liteclient.LiteServerBlockStateC, error)
	GetBlockProofRaw(ctx context.Context, knownBlock ton.BlockIDExt, targetBlock *ton.BlockIDExt) (liteclient.LiteServerPartialBlockProofC, error)
	GetShardBlockProofRaw(ctx context.Context) (liteclient.LiteServerShardBlockProofC, error)
	GetAllShardsInfo(ctx context.Context, blockID ton.BlockIDExt) ([]ton.BlockIDExt, error)
	GetAllShardsInfoRaw(ctx context.Context, blockID ton.BlockIDExt) (liteclient.LiteServerAllShardsInfoC, error)
	GetShardInfoRaw(ctx context.Context, blockID ton.BlockIDExt, workchain uint32, shard uint64, exact bool) (liteclient.LiteServerShardInfoC, error)
	GetOutMsgQueueSizes(ctx context.Context) (liteclient.LiteServerOutMsgQueueSizesC, error)

	// Accounts and transactions.
	GetAccountState(ctx context.Context, accountID ton.AccountID) (tlb.ShardAccount, error)
	GetAccountStateRaw(ctx context.Context, accountID ton.AccountID) (liteclient.LiteServerAccountStateC, error)
	GetSeqno(ctx context.Context, account ton.AccountID) (uint32, error)
	GetLastTransactions(ctx context.Context, a ton.AccountID, limit int) ([]ton.Transaction, error)
	GetTransactionsRaw(ctx context.Context, count uint32, accountID ton.AccountID, lt uint64, hash ton.Bits256) (liteclient.LiteServerTransactionListC, error)
	ListBlockTransactionsRaw(ctx context.Context, blockID ton.BlockIDExt, mode, count uint32, after *liteclient.LiteServerTransactionId3C) (liteclient.LiteServerBlockTransactionsC, error)

	// Config, libraries and the VM.
	GetConfigAll(ctx context.Context, mode liteapi.ConfigMode) (tlb.ConfigParams, error)
	GetConfigAllRaw(ctx context.Context, mode liteapi.ConfigMode) (liteclient.LiteServerConfigInfoC, error)
	GetConfigParams(ctx context.Context, mode liteapi.ConfigMode, paramList []uint32) (tlb.ConfigParams, error)
	GetLibraries(ctx context.Context, libraryList []ton.Bits256) (map[ton.Bits256]*boc.Cell, error)
	GetJettonData(ctx context.Context, master ton.AccountID) (tep64.Metadata, error)
	RunSmcMethod(ctx context.Context, accountID ton.AccountID, method string, params tlb.VmStack) (uint32, tlb.VmStack, error)
	RunSmcMethodByID(ctx context.Context, accountID ton.AccountID, methodID int, params tlb.VmStack) (uint32, tlb.VmStack, error)

	// SendMessage broadcasts an external message.
	SendMessage(ctx context.Context, payload []byte) (uint32, error)

	// WithBlock pins the queries made through the returned client to a block,
	// for the reads that must be answered as of one block rather than as of
	// whenever each of them happened to be served.
	WithBlock(blockID ton.BlockIDExt) LiteClient
}

// liteAPIClient adapts tongo's *liteapi.Client to LiteClient.
//
// Exactly one method needs writing. Go has no covariant returns, so the
// concrete client's WithBlock, which returns *liteapi.Client, cannot satisfy an
// interface method returning LiteClient. Every other method is promoted from
// the embedded client unchanged - the interface was written to tongo's
// signatures so that this stays true.
type liteAPIClient struct {
	*liteapi.Client
}

// LiteAPIClient wraps a tongo lite client so it can be used as a LiteClient.
func LiteAPIClient(cli *liteapi.Client) LiteClient {
	return liteAPIClient{Client: cli}
}

func (c liteAPIClient) WithBlock(blockID ton.BlockIDExt) LiteClient {
	return liteAPIClient{Client: c.Client.WithBlock(blockID)}
}
