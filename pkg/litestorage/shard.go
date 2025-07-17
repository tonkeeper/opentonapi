package litestorage

import (
	"context"
	"github.com/tonkeeper/tongo/ton"
)

func (s *LiteStorage) GetAllShardsInfo(ctx context.Context, blockID ton.BlockIDExt) ([]ton.BlockIDExt, error) {
	return s.client.GetAllShardsInfo(ctx, blockID)
}
