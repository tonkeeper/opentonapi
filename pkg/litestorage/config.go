package litestorage

import (
	"context"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
)

// allowedConfigKeys is a list of blockchain config keys
// we keep in config to optimize the performance of tvm and tvemulator.
var allowedConfigKeys = []uint32{
	0, 1, 2, 3, 4, 5,
	8,
	9, 10,
	12,
	15,
	17,
	18,
	20,
	21,
	24,
	25,
	32, // 32 + 34 together take up to 98% of the config size
	34,
	79, 80, 81, 82, // required by token bridge https://github.com/ton-blockchain/token-bridge-func/blob/3346a901e3e8e1a1e020fac564c845db3220c238/src/func/jetton-bridge/jetton-wallet.fc#L233
}

func (c *LiteStorage) GetLastConfig(ctx context.Context) (ton.BlockchainConfig, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_last_config").Observe(v)
	}))
	defer timer.ObserveDuration()
	config, prs := c.configCache.Get(1)
	if prs {
		return config, nil

	}
	rawConfig, err := c.client.GetConfigAll(ctx, 0)
	if err != nil {
		return ton.BlockchainConfig{}, err
	}
	configP, err := ton.ConvertBlockchainConfigStrict(rawConfig)
	if err != nil {
		return ton.BlockchainConfig{}, err
	}

	c.configCache.Set(1, *configP, cache.WithExpiration(time.Second*2)) //todo: remove
	return *configP, err
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

func (c *LiteStorage) GetConfigRaw(ctx context.Context) ([]byte, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_config_raw").Observe(v)
	}))
	defer timer.ObserveDuration()
	raw, err := c.client.GetConfigAllRaw(ctx, 0)
	if err != nil {
		return nil, err
	}
	return raw.ConfigProof, err

}

// TrimmedConfigBase64 returns the current trimmed blockchain config in a base64 format.
func (c *LiteStorage) TrimmedConfigBase64() (string, error) {
	conf := c.blockchainConfig()
	if len(conf) > 0 {
		return conf, nil
	}
	// we haven't updated the config yet, so let's do it now.
	// this can happen at start up.
	params, err := c.client.GetConfigAll(context.TODO(), 0)
	if err != nil {
		return "", err
	}
	return c.updateBlockchainConfig(params)
}

func (c *LiteStorage) blockchainConfig() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.trimmedConfigBase64
}

func (c *LiteStorage) updateBlockchainConfig(params tlb.ConfigParams) (string, error) {
	params = params.CloneKeepingSubsetOfKeys(allowedConfigKeys)
	cell := boc.NewCell()
	if err := tlb.Marshal(cell, params.Config); err != nil {
		return "", err
	}
	configBase64, err := cell.ToBocBase64()
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.trimmedConfigBase64 = configBase64
	return configBase64, nil
}

func (s *LiteStorage) runBlockchainConfigUpdate(updateInterval time.Duration) {
	go func() {
		for {
			select {
			case <-s.stopCh:
				return
			// TODO: find better way to update config.
			// For example, we can update a config once a new key block is added to the blockchain.
			case <-time.After(updateInterval):
				params, err := s.client.GetConfigAll(context.TODO(), 0)
				if err != nil {
					s.logger.Error("failed to get blockchain config", zap.Error(err))
					continue
				}
				if _, err := s.updateBlockchainConfig(params); err != nil {
					s.logger.Error("failed to get blockchain config", zap.Error(err))
					continue
				}
			}
		}
	}()
}
