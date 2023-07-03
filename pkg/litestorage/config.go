package litestorage

import (
	"context"

	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
)

func (c *LiteStorage) GetLastConfig() ([]byte, error) {
	conf, err := c.client.GetConfigAll(context.Background(), 0)
	if err != nil {
		return nil, err
	}
	cell := boc.NewCell()
	if err := tlb.Marshal(cell, conf.Config); err != nil {
		return nil, err
	}
	bocBytes, err := cell.ToBoc()
	if err != nil {
		return nil, err
	}
	return bocBytes, nil
}
