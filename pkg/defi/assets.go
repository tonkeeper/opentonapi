package defi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
)

func GetDefiAssets(ctx context.Context, d Deps, account tongo.AccountID, rates map[string]float64) *oas.DefiAssets {
	var (
		result oas.DefiAssets
		mu     sync.Mutex
		wg     sync.WaitGroup
	)

	collect := func(name string, fn func() []oas.DefiAsset) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			assets := fn()
			fmt.Println("defi collect", name, "took", time.Since(start))
			if len(assets) == 0 {
				return
			}
			mu.Lock()
			result.Assets = append(result.Assets, assets...)
			mu.Unlock()
		}()
	}

	collect("liquidStakingJettons", func() []oas.DefiAsset { return collectLiquidStakingJettons(ctx, d, account, rates) })
	collect("whalesStaking", func() []oas.DefiAsset { return collectWhalesStaking(ctx, d, account) })
	collect("tfStaking", func() []oas.DefiAsset { return collectTFStaking(ctx, d, account) })
	collect("lpPositions", func() []oas.DefiAsset { return collectLPPositions(ctx, d, account, rates) })
	collect("toncoPositions", func() []oas.DefiAsset { return collectToncoPositions(ctx, d, account) })
	collect("stonfiExternalPositions", func() []oas.DefiAsset { return collectStonfiExternalPositions(ctx, d, account, rates) })
	collect("stonfiStakingPositions", func() []oas.DefiAsset { return collectStonfiStakingPositions(ctx, d, account, rates) })
	collect("swapCoffeePositions", func() []oas.DefiAsset { return collectSwapCoffeePositions(ctx, d, account, rates) })
	collect("evaaPositions", func() []oas.DefiAsset { return collectEvaaPositions(ctx, d, account, rates) })
	collect("affluentPositions", func() []oas.DefiAsset { return collectAffluentPositions(ctx, d, account, rates) })
	// collect("bidaskPositions", func() []oas.DefiAsset { return collectBidaskPositions(ctx, d, account, rates) })

	wg.Wait()

	if result.Assets == nil {
		result.Assets = []oas.DefiAsset{}
	}
	return &result
}
