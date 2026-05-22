package defi

import (
	"context"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
)

func collectWhalesStaking(ctx context.Context, d Deps, account tongo.AccountID) []oas.DefiAsset {
	nominators, err := d.Storage.GetParticipatingInWhalesPools(ctx, account)
	if err != nil {
		return nil
	}
	var assets []oas.DefiAsset
	for _, n := range nominators {
		total := n.MemberBalance + n.MemberPendingDeposit + n.MemberWithdraw
		if total == 0 {
			continue
		}
		assets = append(assets, oas.DefiAsset{
			Type:     oas.DefiAssetTypeStaking,
			Provider: defiProvider(references.WhalesProvider),
			Amount:   total,
			Position: total,
		})
	}
	return assets
}

func collectTFStaking(ctx context.Context, d Deps, account tongo.AccountID) []oas.DefiAsset {
	pools, err := d.Storage.GetParticipatingInTfPools(ctx, account)
	if err != nil {
		return nil
	}
	var assets []oas.DefiAsset
	for _, n := range pools {
		total := n.MemberBalance + n.MemberPendingDeposit + n.MemberWithdraw
		if total == 0 {
			continue
		}
		assets = append(assets, oas.DefiAsset{
			Type:     oas.DefiAssetTypeStaking,
			Provider: defiProvider(references.TFProvider),
			Amount:   total,
			Position: total,
		})
	}
	return assets
}
