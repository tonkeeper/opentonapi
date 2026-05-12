package defi

import (
	"context"
	"math/big"
	"sync"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
)

func collectBidaskPositions(ctx context.Context, d Deps, account tongo.AccountID, rates map[string]float64) []oas.DefiAsset {
	pools, err := d.Storage.BidaskPools(ctx)
	if err != nil || len(pools) == 0 {
		return nil
	}

	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		assets []oas.DefiAsset
	)
	for _, poolAddr := range pools {
		poolAddr := poolAddr
		wg.Add(1)
		go func() {
			defer wg.Done()
			asset, ok := buildBidaskAsset(ctx, d, poolAddr, account, rates)
			if !ok {
				return
			}
			mu.Lock()
			assets = append(assets, asset)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return assets
}

func buildBidaskAsset(ctx context.Context, d Deps, poolAddr tongo.AccountID, account tongo.AccountID, rates map[string]float64) (oas.DefiAsset, bool) {
	_, rangeRaw, err := abi.GetActiveRange(ctx, d.Executor, poolAddr)
	if err != nil {
		return oas.DefiAsset{}, false
	}
	rangeResult, ok := rangeRaw.(abi.GetActiveRange_BidaskResult)
	if !ok {
		return oas.DefiAsset{}, false
	}
	rangeID, err := tongo.AccountIDFromTlb(rangeResult.RangeAddress)
	if err != nil || rangeID == nil {
		return oas.DefiAsset{}, false
	}

	_, multitokenRaw, err := abi.GetLpMultitokenWallet(ctx, d.Executor, *rangeID, account.ToMsgAddress())
	if err != nil {
		return oas.DefiAsset{}, false
	}
	multitokenResult, ok := multitokenRaw.(abi.GetLpMultitokenWallet_BidaskResult)
	if !ok {
		return oas.DefiAsset{}, false
	}
	multitokenID, err := tongo.AccountIDFromTlb(multitokenResult.MultitokenAddress)
	if err != nil || multitokenID == nil {
		return oas.DefiAsset{}, false
	}

	errCode, stack, err := d.Executor.RunSmcMethodByID(ctx, *multitokenID, 75236, tlb.VmStack{})
	if stack.Len() != 1 || (stack.Peek(0).SumType != "VmStkCell") {
		return oas.DefiAsset{}, false
	}
	if errCode != 0 && errCode != 1 {
		return oas.DefiAsset{}, false
	}
	type BidaskMultitokenStorage struct {
		UserAddress  tlb.MsgAddress                        `tlb:"addr"`
		RangeAddress tlb.MsgAddress                        `tlb:"addr"`
		BinsNumber   uint32                                `tlb:"## 32"`
		Tokens       tlb.HashmapE[tlb.Uint32, tlb.Uint128] `tlb:"dict 32"`
	}

	type GetStorage_BidaskMultitokenResult struct {
		Storage BidaskMultitokenStorage `tlb:"^"`
	}

	var result GetStorage_BidaskMultitokenResult
	err = stack.Unmarshal(&result)
	if err != nil {
		return oas.DefiAsset{}, false
	}

	totalLP := new(big.Int)
	for _, item := range result.Storage.Tokens.Items() {
		lpAmount := new(big.Int).Set((*big.Int)(&item.Value))
		totalLP.Add(totalLP, lpAmount)
	}
	if totalLP.Sign() == 0 {
		return oas.DefiAsset{}, false
	}

	amountX, amountY := big.NewInt(0), big.NewInt(0)

	masterX, masterY := bidaskPoolTokenMasters(ctx, d, poolAddr)

	posX := currencyReserveToNanoTON(ctx, d, rates, amountX, bidaskCurrency(masterX))
	posY := currencyReserveToNanoTON(ctx, d, rates, amountY, bidaskCurrency(masterY))
	positionNanoTON := int64(posX + posY)

	asset := oas.DefiAsset{
		Type:     oas.DefiAssetTypeLiquidPool,
		Provider: defiProvider(references.BidaskProvider),
		Amount:   amountX.Int64(),
		Position: positionNanoTON,
	}
	preview := oas.DefiNftPreview{
		Address:    multitokenID.ToRaw(),
		Collection: rangeID.ToRaw(),
	}
	if masterX != nil {
		meta := d.JettonMeta(ctx, *masterX)
		score, _ := d.Score.GetJettonScore(*masterX)
		preview.Token0.SetTo(JettonPreview(*masterX, meta, score))
	}
	if masterY != nil {
		meta := d.JettonMeta(ctx, *masterY)
		score, _ := d.Score.GetJettonScore(*masterY)
		preview.Token1.SetTo(JettonPreview(*masterY, meta, score))
	}
	asset.Nft.SetTo(preview)
	return asset, true
}

func bidaskPoolTokenMasters(ctx context.Context, d Deps, poolAddr tongo.AccountID) (*tongo.AccountID, *tongo.AccountID) {
	_, infoRaw, err := abi.GetPoolInfo(ctx, d.Executor, poolAddr)
	if err != nil {
		return nil, nil
	}

	var walletX, walletY tlb.MsgAddress
	switch v := infoRaw.(type) {
	case abi.GetPoolInfo_BidaskResult:
		walletX, walletY = v.JettonWalletX, v.JettonWalletY
	case abi.GetPoolInfo_BidaskDammResult:
		walletX, walletY = v.JettonWalletX, v.JettonWalletY
	default:
		return nil, nil
	}

	walletXID, err := tongo.AccountIDFromTlb(walletX)
	if err != nil || walletXID == nil {
		return nil, nil
	}
	walletYID, err := tongo.AccountIDFromTlb(walletY)
	if err != nil || walletYID == nil {
		return nil, nil
	}

	masterX := resolveJettonMaster(ctx, d, *walletXID)
	masterY := resolveJettonMaster(ctx, d, *walletYID)
	return masterX, masterY
}

func resolveJettonMaster(ctx context.Context, d Deps, walletAddr tongo.AccountID) *tongo.AccountID {
	_, res, err := abi.GetWalletData(ctx, d.Executor, walletAddr)
	if err != nil {
		return nil
	}
	data, ok := res.(abi.GetWalletDataResult)
	if !ok {
		return nil
	}
	master, err := tongo.AccountIDFromTlb(data.Jetton)
	if err != nil {
		return nil
	}
	return master
}

func bidaskCurrency(master *tongo.AccountID) core.Currency {
	if master == nil {
		return core.Currency{}
	}
	return core.Currency{Type: core.CurrencyJetton, Jetton: master}
}
