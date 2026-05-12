package defi

import (
	"context"
	"math/big"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
	"go.uber.org/zap"
)

func collectAffluentPositions(ctx context.Context, d Deps, account tongo.AccountID, rates map[string]float64) []oas.DefiAsset {
	addrVal, err := tlb.TlbStructToVmCellSlice(account.ToMsgAddress())
	if err != nil {
		d.warn("affluent: failed to build address arg", zap.Error(err))
		return nil
	}

	var assets []oas.DefiAsset
	for _, vaultRaw := range references.AffVaults {
		vaultID, err := tongo.ParseAddress(vaultRaw)
		if err != nil {
			continue
		}
		stack := tlb.VmStack{}
		stack.Put(addrVal)
		_, resultStack, err := d.Executor.RunSmcMethod(ctx, vaultID.ID, "get_user_data", stack)
		if err != nil || resultStack.Len() < 1 {
			continue
		}
		top := resultStack.Peek(0)
		if top.SumType != "VmStkInt" {
			continue
		}
		principal := (*big.Int)(&top.VmStkInt).Int64()
		if principal == 0 {
			continue
		}

		meta := d.JettonMeta(ctx, vaultID.ID)
		score, _ := d.Score.GetJettonScore(vaultID.ID)
		absPrincipal := decimal.NewFromInt(principal).Abs()
		pos := JettonPositionNanoTON(rates, vaultID.ID.ToRaw(), absPrincipal, meta.Decimals)
		if principal < 0 {
			pos = -pos
		}
		asset := oas.DefiAsset{
			Type:     oas.DefiAssetTypeLending,
			Provider: defiProvider(references.AffluentProvider),
			Amount:   principal,
			Position: pos,
		}
		asset.Jetton.SetTo(JettonPreview(vaultID.ID, meta, score))
		assets = append(assets, asset)
	}
	return assets
}
