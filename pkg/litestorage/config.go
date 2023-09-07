package litestorage

import (
	"context"

	"github.com/tonkeeper/tongo/tlb"
)

func (c *LiteStorage) GetLastConfig(ctx context.Context) (tlb.ConfigParams, error) {
	return c.client.GetConfigAll(ctx, 0)
}
