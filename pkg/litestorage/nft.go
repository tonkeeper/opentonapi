package litestorage

import (
	"context"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
)

func (s *LiteStorage) GetNFTs(ctx context.Context, accounts []tongo.AccountID) ([]core.NftItem, error) {
	return nil, nil
}

func (s *LiteStorage) SearchNFTs(ctx context.Context,
	collection *core.Filter[tongo.AccountID],
	owner *core.Filter[tongo.AccountID],
	includeOnSale bool,
	onlyVerified bool,
	limit, offset int,
) ([]tongo.AccountID, error) {
	return nil, nil
}
