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

func (s *LiteStorage) GetNftCollections(ctx context.Context, limit, offset *int32) ([]core.NftCollection, error) {
	return nil, nil
}

func (s *LiteStorage) GetNftCollectionByCollectionAddress(ctx context.Context, address tongo.AccountID) (core.NftCollection, error) {
	return core.NftCollection{}, nil
}
