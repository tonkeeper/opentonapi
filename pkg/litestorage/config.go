package litestorage

import (
	"context"

	"github.com/tonkeeper/tongo/tlb"
)

func (c *LiteStorage) GetLastConfig() (tlb.ConfigParams, error) {
	return c.client.GetConfigAll(context.Background(), 0)
}
