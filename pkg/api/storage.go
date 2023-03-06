package api

import (
	"context"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

type storage interface {
	GetBlockHeader(ctx context.Context, id tongo.BlockID) (*core.BlockHeader, error)
	GetTransaction(ctx context.Context, hash tongo.Bits256) (*core.Transaction, error)
	GetTrace(ctx context.Context, hash tongo.Bits256) (*core.Trace, error)
	GetParticipatingInWhalesPools(ctx context.Context, id tongo.AccountID) ([]core.WhalesNominator, error)
	GetWhalesPoolInfo(ctx context.Context, id tongo.AccountID) (abi.GetParams_WhalesNominatorResult, abi.GetStakingStatusResult, error)
}

type chainState interface {
	GetAPY() float64
}
