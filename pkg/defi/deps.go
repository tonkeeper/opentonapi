package defi

import (
	"context"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
)

type Deps struct {
	Storage    Storage
	Executor   Executor
	Score      Score
	Logger     *zap.Logger
	JettonMeta func(context.Context, tongo.AccountID) NormalizedJettonMeta
}

type Storage interface {
	GetJettonWalletsByOwnerAddress(ctx context.Context, address tongo.AccountID, jetton *tongo.AccountID, isJettonMaster bool, mintless bool, limit, offset int) ([]core.JettonWallet, error)
	DedustPools(ctx context.Context, contracts []tongo.AccountID) (map[tongo.AccountID]core.DedustPool, error)
	GetJettonMasterData(ctx context.Context, master tongo.AccountID) (core.JettonMaster, error)
	BidaskPools(ctx context.Context) ([]tongo.AccountID, error)
	SearchNFTs(ctx context.Context, collection *core.Filter[tongo.AccountID], owner *core.Filter[tongo.AccountID], includeOnSale bool, onlyVerified bool, limit, offset int) ([]tongo.AccountID, error)
	GetNFTs(ctx context.Context, accounts []tongo.AccountID) ([]core.NftItem, error)
	GetParticipatingInWhalesPools(ctx context.Context, id tongo.AccountID) ([]core.Nominator, error)
	GetParticipatingInTfPools(ctx context.Context, member tongo.AccountID) ([]core.Nominator, error)
}

type Executor interface {
	RunSmcMethod(context.Context, tongo.AccountID, string, tlb.VmStack) (uint32, tlb.VmStack, error)
	RunSmcMethodByID(context.Context, tongo.AccountID, int, tlb.VmStack) (uint32, tlb.VmStack, error)
}

type Score interface {
	GetJettonScore(masterID ton.AccountID) (int32, error)
}

func (d Deps) warn(msg string, fields ...zap.Field) {
	if d.Logger == nil {
		return
	}
	d.Logger.Warn(msg, fields...)
}

func (d Deps) info(msg string, fields ...zap.Field) {
	if d.Logger == nil {
		return
	}
	d.Logger.Info(msg, fields...)
}
