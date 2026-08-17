package service

import (
	"context"

	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/liteclient"
	"github.com/tonkeeper/tongo/tep64"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// The methods in this file complete RoundRobinClient's LiteClient contract.
// The rewards service does not call any of them - the ones it does call live in
// roundrobin.go, instrumented with the attributes that are worth having for
// them. These exist so that a RoundRobinClient is a whole blockchain client and
// not a special case, which is what lets the service take the same LiteClient
// as everything else.
//
// They are deliberately uniform: pick a connection, delegate, record the error.
// Anything that needs more than that belongs in roundrobin.go with the rest.

// delegate runs one pass-through call inside a span named for the method,
// noting the pinned block when the client is pinned to one.
func delegate[T any](ctx context.Context, r *RoundRobinClient, method string, fn func(context.Context, *liteapi.Client) (T, error)) (T, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "liteclient."+method, trace.WithSpanKind(trace.SpanKindClient))
	if r.targetBlock != nil {
		span.SetAttributes(attribute.Int64("ton.block.seqno", int64(r.targetBlock.BlockID.Seqno)))
	}
	defer span.End()
	res, err := fn(ctx, r.clientForRequest(ctx))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return res, err
}

func (r *RoundRobinClient) GetMasterchainInfoExt(ctx context.Context, mode uint32) (liteclient.LiteServerMasterchainInfoExtC, error) {
	return delegate(ctx, r, "GetMasterchainInfoExt", func(ctx context.Context, c *liteapi.Client) (liteclient.LiteServerMasterchainInfoExtC, error) {
		return c.GetMasterchainInfoExt(ctx, mode)
	})
}

func (r *RoundRobinClient) GetTime(ctx context.Context) (uint32, error) {
	return delegate(ctx, r, "GetTime", func(ctx context.Context, c *liteapi.Client) (uint32, error) {
		return c.GetTime(ctx)
	})
}

func (r *RoundRobinClient) GetBlockRaw(ctx context.Context, blockID ton.BlockIDExt) (liteclient.LiteServerBlockDataC, error) {
	return delegate(ctx, r, "GetBlockRaw", func(ctx context.Context, c *liteapi.Client) (liteclient.LiteServerBlockDataC, error) {
		return c.GetBlockRaw(ctx, blockID)
	})
}

func (r *RoundRobinClient) GetBlockHeaderRaw(ctx context.Context, blockID ton.BlockIDExt, mode uint32) (liteclient.LiteServerBlockHeaderC, error) {
	return delegate(ctx, r, "GetBlockHeaderRaw", func(ctx context.Context, c *liteapi.Client) (liteclient.LiteServerBlockHeaderC, error) {
		return c.GetBlockHeaderRaw(ctx, blockID, mode)
	})
}

func (r *RoundRobinClient) GetStateRaw(ctx context.Context, blockID ton.BlockIDExt) (liteclient.LiteServerBlockStateC, error) {
	return delegate(ctx, r, "GetStateRaw", func(ctx context.Context, c *liteapi.Client) (liteclient.LiteServerBlockStateC, error) {
		return c.GetStateRaw(ctx, blockID)
	})
}

func (r *RoundRobinClient) GetBlockProofRaw(ctx context.Context, knownBlock ton.BlockIDExt, targetBlock *ton.BlockIDExt) (liteclient.LiteServerPartialBlockProofC, error) {
	return delegate(ctx, r, "GetBlockProofRaw", func(ctx context.Context, c *liteapi.Client) (liteclient.LiteServerPartialBlockProofC, error) {
		return c.GetBlockProofRaw(ctx, knownBlock, targetBlock)
	})
}

func (r *RoundRobinClient) GetShardBlockProofRaw(ctx context.Context) (liteclient.LiteServerShardBlockProofC, error) {
	return delegate(ctx, r, "GetShardBlockProofRaw", func(ctx context.Context, c *liteapi.Client) (liteclient.LiteServerShardBlockProofC, error) {
		return c.GetShardBlockProofRaw(ctx)
	})
}

func (r *RoundRobinClient) GetAllShardsInfo(ctx context.Context, blockID ton.BlockIDExt) ([]ton.BlockIDExt, error) {
	return delegate(ctx, r, "GetAllShardsInfo", func(ctx context.Context, c *liteapi.Client) ([]ton.BlockIDExt, error) {
		return c.GetAllShardsInfo(ctx, blockID)
	})
}

func (r *RoundRobinClient) GetAllShardsInfoRaw(ctx context.Context, blockID ton.BlockIDExt) (liteclient.LiteServerAllShardsInfoC, error) {
	return delegate(ctx, r, "GetAllShardsInfoRaw", func(ctx context.Context, c *liteapi.Client) (liteclient.LiteServerAllShardsInfoC, error) {
		return c.GetAllShardsInfoRaw(ctx, blockID)
	})
}

func (r *RoundRobinClient) GetShardInfoRaw(ctx context.Context, blockID ton.BlockIDExt, workchain uint32, shard uint64, exact bool) (liteclient.LiteServerShardInfoC, error) {
	return delegate(ctx, r, "GetShardInfoRaw", func(ctx context.Context, c *liteapi.Client) (liteclient.LiteServerShardInfoC, error) {
		return c.GetShardInfoRaw(ctx, blockID, workchain, shard, exact)
	})
}

func (r *RoundRobinClient) GetOutMsgQueueSizes(ctx context.Context) (liteclient.LiteServerOutMsgQueueSizesC, error) {
	return delegate(ctx, r, "GetOutMsgQueueSizes", func(ctx context.Context, c *liteapi.Client) (liteclient.LiteServerOutMsgQueueSizesC, error) {
		return c.GetOutMsgQueueSizes(ctx)
	})
}

func (r *RoundRobinClient) GetAccountStateRaw(ctx context.Context, accountID ton.AccountID) (liteclient.LiteServerAccountStateC, error) {
	return delegate(ctx, r, "GetAccountStateRaw", func(ctx context.Context, c *liteapi.Client) (liteclient.LiteServerAccountStateC, error) {
		return c.GetAccountStateRaw(ctx, accountID)
	})
}

func (r *RoundRobinClient) GetSeqno(ctx context.Context, account ton.AccountID) (uint32, error) {
	return delegate(ctx, r, "GetSeqno", func(ctx context.Context, c *liteapi.Client) (uint32, error) {
		return c.GetSeqno(ctx, account)
	})
}

func (r *RoundRobinClient) GetLastTransactions(ctx context.Context, a ton.AccountID, limit int) ([]ton.Transaction, error) {
	return delegate(ctx, r, "GetLastTransactions", func(ctx context.Context, c *liteapi.Client) ([]ton.Transaction, error) {
		return c.GetLastTransactions(ctx, a, limit)
	})
}

func (r *RoundRobinClient) GetTransactionsRaw(ctx context.Context, count uint32, accountID ton.AccountID, lt uint64, hash ton.Bits256) (liteclient.LiteServerTransactionListC, error) {
	return delegate(ctx, r, "GetTransactionsRaw", func(ctx context.Context, c *liteapi.Client) (liteclient.LiteServerTransactionListC, error) {
		return c.GetTransactionsRaw(ctx, count, accountID, lt, hash)
	})
}

func (r *RoundRobinClient) ListBlockTransactionsRaw(ctx context.Context, blockID ton.BlockIDExt, mode, count uint32, after *liteclient.LiteServerTransactionId3C) (liteclient.LiteServerBlockTransactionsC, error) {
	return delegate(ctx, r, "ListBlockTransactionsRaw", func(ctx context.Context, c *liteapi.Client) (liteclient.LiteServerBlockTransactionsC, error) {
		return c.ListBlockTransactionsRaw(ctx, blockID, mode, count, after)
	})
}

func (r *RoundRobinClient) GetConfigAll(ctx context.Context, mode liteapi.ConfigMode) (tlb.ConfigParams, error) {
	return delegate(ctx, r, "GetConfigAll", func(ctx context.Context, c *liteapi.Client) (tlb.ConfigParams, error) {
		return c.GetConfigAll(ctx, mode)
	})
}

func (r *RoundRobinClient) GetConfigAllRaw(ctx context.Context, mode liteapi.ConfigMode) (liteclient.LiteServerConfigInfoC, error) {
	return delegate(ctx, r, "GetConfigAllRaw", func(ctx context.Context, c *liteapi.Client) (liteclient.LiteServerConfigInfoC, error) {
		return c.GetConfigAllRaw(ctx, mode)
	})
}

func (r *RoundRobinClient) GetLibraries(ctx context.Context, libraryList []ton.Bits256) (map[ton.Bits256]*boc.Cell, error) {
	return delegate(ctx, r, "GetLibraries", func(ctx context.Context, c *liteapi.Client) (map[ton.Bits256]*boc.Cell, error) {
		return c.GetLibraries(ctx, libraryList)
	})
}

func (r *RoundRobinClient) GetJettonData(ctx context.Context, master ton.AccountID) (tep64.Metadata, error) {
	return delegate(ctx, r, "GetJettonData", func(ctx context.Context, c *liteapi.Client) (tep64.Metadata, error) {
		return c.GetJettonData(ctx, master)
	})
}

func (r *RoundRobinClient) SendMessage(ctx context.Context, payload []byte) (uint32, error) {
	return delegate(ctx, r, "SendMessage", func(ctx context.Context, c *liteapi.Client) (uint32, error) {
		return c.SendMessage(ctx, payload)
	})
}
