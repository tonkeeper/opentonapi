package defi

import (
	"context"
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
)

func collectToncoPositions(ctx context.Context, d Deps, account tongo.AccountID) []oas.DefiAsset {
	collectionFilter := &core.Filter[tongo.AccountID]{Value: references.ToncoNFTCollection}
	ownerFilter := &core.Filter[tongo.AccountID]{Value: account}

	nftAddresses, err := d.Storage.SearchNFTs(ctx, collectionFilter, ownerFilter, false, false, 100, 0)
	if err != nil || len(nftAddresses) == 0 {
		return nil
	}
	nfts, err := d.Storage.GetNFTs(ctx, nftAddresses)
	if err != nil {
		return nil
	}

	assets := make([]oas.DefiAsset, 0, len(nfts))
	for _, nft := range nfts {
		if nft.CollectionAddress == nil {
			continue
		}
		liquidity, token0ID, token1ID := toncoPositionInfo(ctx, d, nft.Address, *nft.CollectionAddress)
		asset := oas.DefiAsset{
			Type:     oas.DefiAssetTypeLiquidPool,
			Provider: defiProvider(references.ToncoProvider),
			Amount:   liquidity,
			Position: 0, // position in nanoTON requires current sqrt price — not computed here
		}
		preview := oas.DefiNftPreview{
			Address:    nft.Address.ToRaw(),
			Collection: nft.CollectionAddress.ToRaw(),
		}
		if token0ID != nil {
			meta0 := d.JettonMeta(ctx, *token0ID)
			score0, _ := d.Score.GetJettonScore(*token0ID)
			preview.Token0.SetTo(JettonPreview(*token0ID, meta0, score0))
		}
		if token1ID != nil {
			meta1 := d.JettonMeta(ctx, *token1ID)
			score1, _ := d.Score.GetJettonScore(*token1ID)
			preview.Token1.SetTo(JettonPreview(*token1ID, meta1, score1))
		}
		asset.Nft.SetTo(preview)
		assets = append(assets, asset)
	}
	return assets
}

func toncoPositionInfo(ctx context.Context, d Deps, nftAddr tongo.AccountID, collectionAddr tongo.AccountID) (int64, *tongo.AccountID, *tongo.AccountID) {
	_, posStack, err := d.Executor.RunSmcMethod(ctx, nftAddr, "getPositionInfo", tlb.VmStack{})
	var liquidity int64
	if err == nil && posStack.Len() >= 5 {
		v := posStack.Peek(2)
		if v.SumType == "VmStkInt" {
			liquidity = (*big.Int)(&v.VmStkInt).Int64()
		}
	}

	var token0, token1 *tongo.AccountID
	_, poolStack, err := d.Executor.RunSmcMethod(ctx, collectionAddr, "getPoolData", tlb.VmStack{})
	if err == nil && poolStack.Len() >= 2 {
		for i, idx := range []int{0, 1} {
			v := poolStack.Peek(idx)
			if v.SumType != "VmStkSlice" {
				continue
			}
			var addr tlb.MsgAddress
			if err := v.VmStkSlice.UnmarshalToTlbStruct(&addr); err != nil {
				continue
			}
			accountID, err := tongo.AccountIDFromTlb(addr)
			if err != nil || accountID == nil {
				continue
			}
			if i == 0 {
				token0 = accountID
			} else {
				token1 = accountID
			}
		}
	}

	return liquidity, token0, token1
}
