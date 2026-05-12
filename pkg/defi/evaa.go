package defi

import (
	"context"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
	"go.uber.org/zap"
)

func collectEvaaPositions(ctx context.Context, d Deps, account tongo.AccountID, rates map[string]float64) []oas.DefiAsset {
	addrArg, err := tlb.TlbStructToVmCellSlice(account.ToMsgAddress())
	if err != nil {
		return nil
	}

	stack1 := tlb.VmStack{}
	stack1.Put(addrArg)
	_, res1, err := d.Executor.RunSmcMethod(ctx, references.EvaaMasterAddress, "get_user_address", stack1)
	if err != nil || res1.Len() < 1 {
		return nil
	}
	top1 := res1.Peek(0)
	if top1.SumType != "VmStkSlice" {
		return nil
	}
	var userContractMsgAddr tlb.MsgAddress
	if err := top1.VmStkSlice.UnmarshalToTlbStruct(&userContractMsgAddr); err != nil {
		return nil
	}
	userContractID, err := tongo.AccountIDFromTlb(userContractMsgAddr)
	if err != nil || userContractID == nil {
		return nil
	}

	_, res2, err := d.Executor.RunSmcMethod(ctx, *userContractID, "get_user_data", tlb.VmStack{})
	if err != nil || res2.Len() < 1 {
		return nil
	}
	top2 := res2.Peek(res2.Len() - 1)
	if top2.SumType != "VmStkCell" {
		return nil
	}

	var principals tlb.HashmapE[tlb.Bits256, tlb.SignedCoins]
	cell := top2.VmStkCell.Value
	if err := tlb.Unmarshal(&cell, &principals); err != nil {
		d.warn("evaa: failed to parse principals", zap.Error(err))
		return nil
	}

	var assets []oas.DefiAsset
	for _, item := range principals.Items() {
		principal := int64(item.Value)
		if principal == 0 {
			continue
		}
		jettonMaster := tongo.AccountID{Workchain: 0, Address: [32]byte(item.Key)}
		meta := d.JettonMeta(ctx, jettonMaster)
		score, _ := d.Score.GetJettonScore(jettonMaster)
		absVal := decimal.NewFromInt(principal).Abs()
		pos := JettonPositionNanoTON(rates, jettonMaster.ToRaw(), absVal, meta.Decimals)
		if principal < 0 {
			pos = -pos
		}
		asset := oas.DefiAsset{
			Type:     oas.DefiAssetTypeLending,
			Provider: defiProvider(references.EvaaProvider),
			Amount:   principal,
			Position: pos,
		}
		asset.Jetton.SetTo(JettonPreview(jettonMaster, meta, score))
		assets = append(assets, asset)
	}
	return assets
}
