package api

import (
	"context"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
)

type storage interface {
	GetBlockHeader(ctx context.Context, id tongo.BlockID) (*core.BlockHeader, error)
	GetTransaction(ctx context.Context, hash tongo.Bits256) (*core.Transaction, error)
	GetTrace(ctx context.Context, hash tongo.Bits256) (*core.Trace, error)
}
