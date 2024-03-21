package litestorage

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

func (c *LiteStorage) GetLastConfig(ctx context.Context) (tlb.ConfigParams, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_last_config").Observe(v)
	}))
	defer timer.ObserveDuration()
	return c.client.GetConfigAll(ctx, 0)
}

func (c *LiteStorage) GetConfigFromBlock(ctx context.Context, id ton.BlockID) (tlb.ConfigParams, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_config_from_block").Observe(v)
	}))
	defer timer.ObserveDuration()
	extID, info, err := c.client.LookupBlock(ctx, id, 1, nil, nil)
	if err != nil {
		return tlb.ConfigParams{}, err
	}
	if !info.KeyBlock {
		return tlb.ConfigParams{}, core.ErrNotKeyBlock
	}
	cli := c.client.WithBlock(extID)
	return cli.GetConfigAll(ctx, 0)
}
