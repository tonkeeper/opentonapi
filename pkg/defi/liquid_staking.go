package defi

import (
	"context"
	"sync"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
)

func collectLiquidStakingJettons(ctx context.Context, d Deps, account tongo.AccountID, rates map[string]float64) []oas.DefiAsset {
	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		assets []oas.DefiAsset
	)
	for _, p := range references.LiquidStakingJettons {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			master := p.JettonMaster
			wallets, err := d.Storage.GetJettonWalletsByOwnerAddress(ctx, account, &master, true, true, 1, 0)
			if err != nil || len(wallets) == 0 || wallets[0].Balance.IsZero() {
				return
			}
			w := wallets[0]
			meta := d.JettonMeta(ctx, master)
			score, _ := d.Score.GetJettonScore(master)
			asset := oas.DefiAsset{
				Type:     oas.DefiAssetTypeLiquidStaking,
				Provider: defiProvider(p.DefiProviderMeta),
				Amount:   w.Balance.BigInt().Int64(),
				Position: JettonPositionNanoTON(rates, master.ToRaw(), w.Balance, meta.Decimals),
			}
			asset.Jetton.SetTo(JettonPreview(master, meta, score))
			mu.Lock()
			assets = append(assets, asset)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return assets
}
