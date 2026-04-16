package model

import (
	"context"
	"sync/atomic"
)

type rpcCounterKey struct{}

// WithRPCCounter returns a child context that tracks RPC call count.
func WithRPCCounter(ctx context.Context) context.Context {
	return context.WithValue(ctx, rpcCounterKey{}, new(atomic.Int64))
}

// RPCCount returns the number of RPC calls recorded in ctx, or 0 if no counter is set.
func RPCCount(ctx context.Context) int64 {
	if c, ok := ctx.Value(rpcCounterKey{}).(*atomic.Int64); ok {
		return c.Load()
	}
	return 0
}

// CountRPC increments the RPC counter in ctx by 1.
func CountRPC(ctx context.Context) {
	if c, ok := ctx.Value(rpcCounterKey{}).(*atomic.Int64); ok {
		c.Add(1)
	}
}
