package service

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"

	"github.com/tonkeeper/tongo/config"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/liteclient"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/tonkeeper/opentonapi/pkg/validatorsrewards/service/liteclient"

// LiteClient is the interface for blockchain operations. Implemented by
// RoundRobinClient and used by all service methods.
type LiteClient interface {
	GetMasterchainInfo(context.Context) (liteclient.LiteServerMasterchainInfoC, error)
	LookupBlock(context.Context, ton.BlockID, uint32, *uint64, *uint32) (ton.BlockIDExt, tlb.BlockInfo, error)
	GetBlock(context.Context, ton.BlockIDExt) (tlb.Block, error)
	GetAccountState(context.Context, ton.AccountID) (tlb.ShardAccount, error)
	GetConfigParams(context.Context, liteapi.ConfigMode, []uint32) (tlb.ConfigParams, error)
	RunSmcMethodByID(context.Context, ton.AccountID, int, tlb.VmStack) (uint32, tlb.VmStack, error)
	RunSmcMethod(context.Context, ton.AccountID, string, tlb.VmStack) (uint32, tlb.VmStack, error)
	WithBlock(ton.BlockIDExt) LiteClient
}

// clientEntry holds a liteapi client and its metadata for debug logging.
type clientEntry struct {
	client *liteapi.Client
	host   string
}

// RoundRobinClient wraps multiple liteapi clients (one per liteserver) and
// distributes requests across them via round-robin. Unlike the default
// liteapi pool which routes all traffic through a single "best" connection,
// this ensures all available connections are utilized.
type RoundRobinClient struct {
	entries     []clientEntry
	counter     uint64
	targetBlock *ton.BlockIDExt
}

// NewRoundRobinClient creates a client that uses one liteapi connection per
// liteserver and round-robins requests across them.
func NewRoundRobinClient(liteServers []config.LiteServer) (*RoundRobinClient, error) {
	entries := make([]clientEntry, 0, len(liteServers))
	for _, server := range liteServers {
		cli, err := liteapi.NewClient(
			liteapi.WithLiteServers([]config.LiteServer{server}),
			liteapi.WithMaxConnectionsNumber(1),
			liteapi.WithAsyncConnectionsInit(),
			liteapi.WithWorkersPerConnection(1),
		)
		if err != nil {
			// Log but continue - we want as many connections as possible
			log.Printf("warning: failed to connect to liteserver %s: %v", server.Host, err)
			continue
		}
		entries = append(entries, clientEntry{client: cli, host: server.Host})
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no liteservers available")
	}

	log.Printf("round-robin client: %d liteserver connections", len(entries))
	return &RoundRobinClient{entries: entries}, nil
}

// hasConnections returns true if the client has at least one working connection.
func hasConnections(c *liteapi.Client) bool {
	status := c.GetPoolStatus()
	for _, conn := range status.Connections {
		if conn.Connected {
			return true
		}
	}
	return false
}

func (r *RoundRobinClient) nextClient(ctx context.Context) (clientEntry, int) {
	entries := r.entries
	n := len(entries)
	if n == 0 {
		return clientEntry{}, -1
	}
	var excluded map[int]bool
	if v := ctx.Value(retryExcludeKey{}); v != nil {
		ex := v.(*RetryExclude)
		ex.mu.Lock()
		excluded = make(map[int]bool)
		for k, v := range ex.Excluded {
			excluded[k] = v
		}
		ex.mu.Unlock()
	}
	// Build list of healthy client indices, excluding any in the retry exclude list.
	healthy := make([]int, 0, n)
	for i, e := range entries {
		if excluded[i] {
			continue
		}
		if hasConnections(e.client) {
			healthy = append(healthy, i)
		}
	}
	if len(healthy) == 0 {
		idx := int(atomic.AddUint64(&r.counter, 1)) % n
		return entries[idx], idx
	}
	// Round-robin among healthy, non-excluded clients only.
	pos := int(atomic.AddUint64(&r.counter, 1)) % len(healthy)
	idx := healthy[pos]
	return entries[idx], idx
}

func (r *RoundRobinClient) clientForRequest(ctx context.Context) *liteapi.Client {
	e, id := r.nextClient(ctx)
	// log.Printf("lite request client_id=%d server=%s", id, e.host)
	if v := ctx.Value(retryExcludeKey{}); v != nil {
		ex := v.(*RetryExclude)
		ex.mu.Lock()
		ex.LastUsed = id
		ex.mu.Unlock()
	}
	c := e.client
	if r.targetBlock != nil {
		return c.WithBlock(*r.targetBlock)
	}
	return c
}

// WithBlock returns a client pinned to the given block for block-specific queries.
func (r *RoundRobinClient) WithBlock(block ton.BlockIDExt) LiteClient {
	return &RoundRobinClient{
		entries:     r.entries,
		counter:     r.counter,
		targetBlock: &block,
	}
}

func (r *RoundRobinClient) GetMasterchainInfo(ctx context.Context) (liteclient.LiteServerMasterchainInfoC, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "liteclient.GetMasterchainInfo", trace.WithSpanKind(trace.SpanKindClient))
	if r.targetBlock != nil {
		span.SetAttributes(attribute.Int64("ton.block.seqno", int64(r.targetBlock.BlockID.Seqno)))
	}
	defer span.End()
	res, err := r.clientForRequest(ctx).GetMasterchainInfo(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return res, err
}

func (r *RoundRobinClient) LookupBlock(ctx context.Context, blockID ton.BlockID, mode uint32, lt *uint64, utime *uint32) (ton.BlockIDExt, tlb.BlockInfo, error) {

	ctx, span := otel.Tracer(tracerName).Start(ctx, "liteclient.LookupBlock", trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.Int64("ton.block.workchain", int64(blockID.Workchain)),
			attribute.Int64("ton.block.shard", int64(blockID.Shard)),
			attribute.Int64("ton.block.seqno", int64(blockID.Seqno)),
			attribute.Int64("ton.lookup.mode", int64(mode)),
		))
	if r.targetBlock != nil {
		span.SetAttributes(attribute.Int64("ton.block.seqno", int64(r.targetBlock.BlockID.Seqno)))
	}
	if utime != nil {
		span.SetAttributes(attribute.Int64("ton.lookup.utime", int64(*utime)))
	}
	if lt != nil {
		span.SetAttributes(attribute.Int64("ton.lookup.lt", int64(*lt)))
	}
	defer span.End()
	ext, info, err := r.clientForRequest(ctx).LookupBlock(ctx, blockID, mode, lt, utime)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return ext, info, err
}

func (r *RoundRobinClient) GetBlock(ctx context.Context, blockID ton.BlockIDExt) (tlb.Block, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "liteclient.GetBlock", trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.Int64("ton.block.workchain", int64(blockID.Workchain)),
			attribute.Int64("ton.block.shard", int64(blockID.Shard)),
			attribute.Int64("ton.block.seqno", int64(blockID.Seqno)),
		))
	if r.targetBlock != nil {
		span.SetAttributes(attribute.Int64("ton.block.seqno", int64(r.targetBlock.BlockID.Seqno)))
	}
	defer span.End()
	block, err := r.clientForRequest(ctx).GetBlock(ctx, blockID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return block, err
}

func (r *RoundRobinClient) GetAccountState(ctx context.Context, accountID ton.AccountID) (tlb.ShardAccount, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "liteclient.GetAccountState", trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("ton.account", accountID.String())))
	if r.targetBlock != nil {
		span.SetAttributes(attribute.Int64("ton.block.seqno", int64(r.targetBlock.BlockID.Seqno)))
	}
	defer span.End()
	state, err := r.clientForRequest(ctx).GetAccountState(ctx, accountID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return state, err
}

func (r *RoundRobinClient) GetConfigParams(ctx context.Context, mode liteapi.ConfigMode, paramList []uint32) (tlb.ConfigParams, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "liteclient.GetConfigParams", trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.Int("ton.config.mode", int(mode)),
			attribute.IntSlice("ton.config.params", intSlice(paramList)),
		))

	if r.targetBlock != nil {
		span.SetAttributes(attribute.Int64("ton.block.seqno", int64(r.targetBlock.BlockID.Seqno)))
	}
	defer span.End()
	params, err := r.clientForRequest(ctx).GetConfigParams(ctx, mode, paramList)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return params, err
}

func (r *RoundRobinClient) RunSmcMethodByID(ctx context.Context, accountID ton.AccountID, methodID int, params tlb.VmStack) (uint32, tlb.VmStack, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "liteclient.RunSmcMethodByID", trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("ton.account", accountID.String()),
			attribute.Int("ton.method.id", methodID),
		))
	if r.targetBlock != nil {
		span.SetAttributes(attribute.Int64("ton.block.seqno", int64(r.targetBlock.BlockID.Seqno)))
	}
	defer span.End()
	exitCode, stack, err := r.clientForRequest(ctx).RunSmcMethodByID(ctx, accountID, methodID, params)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetAttributes(attribute.Int("ton.method.exit_code", int(exitCode)))
	}
	return exitCode, stack, err
}

func (r *RoundRobinClient) RunSmcMethod(ctx context.Context, accountID ton.AccountID, method string, params tlb.VmStack) (uint32, tlb.VmStack, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "liteclient.RunSmcMethod", trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("ton.account", accountID.String()),
			attribute.String("ton.method", method),
		))
	if r.targetBlock != nil {
		span.SetAttributes(attribute.Int64("ton.block.seqno", int64(r.targetBlock.BlockID.Seqno)))
	}
	defer span.End()
	exitCode, stack, err := r.clientForRequest(ctx).RunSmcMethod(ctx, accountID, method, params)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetAttributes(attribute.Int("ton.method.exit_code", int(exitCode)))
	}
	return exitCode, stack, err
}

func intSlice(u []uint32) []int {
	if u == nil {
		return nil
	}
	s := make([]int, len(u))
	for i, v := range u {
		s[i] = int(v)
	}
	return s
}
