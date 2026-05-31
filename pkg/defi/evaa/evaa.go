package evaa

import (
	"context"
	"crypto/sha256"
	"math/big"

	"github.com/tonkeeper/opentonapi/pkg/defi"
	abiEvaa "github.com/tonkeeper/tongo/abi-tolk/abiGenerated/evaa"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
)

type poolInfo struct {
	id   ton.AccountID
	name string
}

var pools = []poolInfo{
	{id: ton.MustParseAccountID("0:BCAD466A47FA565750729565253CD073CA24D856804499090C2100D95C809F9E"), name: "EVAA Main Pool"},
	{id: ton.MustParseAccountID("0:489595F65115A45C24A0DD0176309654FB00B95E40682F0C3E85D5A4D86DFB25"), name: "EVAA LP Pool"},
	{id: ton.MustParseAccountID("0:0D511552DDF8413BD6E2BE2837E22C89422F7B16131BA62BE8D5A504012D8661"), name: "EVAA Alts Pool"},
	{id: ton.MustParseAccountID("0:9D21D5DFD6403FD8777D99B1B34850C43C0F8FC7E7ADF2A4D61C45E0446A342B"), name: "EVAA Stable Pool"},
}

// assets EVAA asset id is sha256 of the ticker
// https://github.com/evaafi/evaa-go-sdk/blob/a9acf74075f5400eec330c7837769b075dd79aac/config/config.go
var assets = map[tlb.Bits256]*ton.AccountID{
	assetID("TON"):                 nil,
	assetID("GRAM"):                nil,
	assetID("USDT"):                mustAccountIDptr("EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sDs"),
	assetID("jUSDT"):               mustAccountIDptr("EQBynBO23ywHy_CgarY9NK9FTz0yDsG82PtcbSTQgGoXwiuA"),
	assetID("jUSDC"):               mustAccountIDptr("EQB-MPwrd1G6WKNkLz_VnV6WqBDd142KMQv-g1O-8QUA3728"),
	assetID("stTON"):               mustAccountIDptr("EQDNhy-nxYFgUqzfUzImBEP67JqsyMIcyk2S5_RwNNEYku0k"),
	assetID("tsTON"):               mustAccountIDptr("EQC98_qAmNEptUtPc7W6xdHh_ZHrBUFpw5Ft_IzNU20QAJav"),
	assetID("USDe"):                mustAccountIDptr("EQAIb6KmdfdDR7CN1GBqVJuP25iCnLKCvBlJ07Evuu2dzP5f"),
	assetID("tsUSDe"):              mustAccountIDptr("EQDQ5UUyPHrLcQJlPAczd_fjxn8SLrlNQwolBznxCdSlfQwr"),
	assetID("TON_STORM"):           mustAccountIDptr("EQCNY2AQ3ZDYwJAqx_nzl9i9Xhd_Ex7izKJM6JTxXRnO6n1F"),
	assetID("USDT_STORM"):          mustAccountIDptr("EQCup4xxCulCcNwmOocM9HtDYPU8xe0449tQLp6a-5BLEegW"),
	assetID("TONUSDT_DEDUST"):      mustAccountIDptr("EQA-X_yo3fzzbDbJ_0bzFWKqtRuZFIRa1sJsveZJ1YpViO3r"),
	assetID("TONUSDT_STONFI"):      mustAccountIDptr("EQCGScrZe1xbyWqWDvdI6mzP-GAcAWFv6ZXuaJOuSqemxku4"),
	assetID("CATI"):                mustAccountIDptr("EQD-cvR0Nz6XAyRBvbhz-abTrRC6sI5tvHvvpeQraV9UAAD7"),
	assetID("DOGS"):                mustAccountIDptr("EQCvxJy4eG8hyHBFsZ7eePxrRsUQSFE_jpptRAYBmcG_DOGS"),
	assetID("NOT"):                 mustAccountIDptr("EQAvlWFDxGF2lXm67y4yzC17wYKD9A0guwPkMs1gOsM__NOT"),
	assetID("STON"):                mustAccountIDptr("EQA2kCVNwVsil2EM2mB0SkXytxCqQjS4mttjDpnXmwG9T6bO"),
	assetID("PT_tsUSDe_01Sep2025"): mustAccountIDptr("EQDb90Bss5FnIyq7VMmnG2UeZIzZomQsILw9Hjo1wxaF1df3"),
	assetID("PT_tsUSDe_18Dec2025"): mustAccountIDptr("EQBxIkea3baUXLtPVuVaSMsWIkC5S0It3OcM8MeYpPnoEWeM"),
}

// Assets returns the EVAA lending positions (supply and borrow) of an account
func Assets(ctx context.Context, executor abiEvaa.Executor, logger *zap.Logger, accountID ton.AccountID) []defi.Asset {
	if executor == nil {
		return nil
	}
	provider, ok := defi.GetProvider("evaa")
	if !ok {
		return nil
	}

	var result []defi.Asset
	for _, p := range pools {
		// with subaccountID argument, same as GetUserAddress, but I wanted to highlight this possibility
		// though that was never observed in the wild
		userAsset, err := abiEvaa.GetUserSubaccountAddress(ctx, executor, p.id, accountID.ToInternal(), 0)
		if err != nil {
			logger.Warn("failed to get evaa user subaccount", zap.String("pool", p.id.ToRaw()), zap.Error(err))
			continue
		}
		userAssetID := ton.AccountID{Workchain: int32(userAsset.Workchain), Address: userAsset.Address}

		principals, err := abiEvaa.GetPrincipals(ctx, executor, userAssetID)
		if err != nil {
			// no log, because in most of the cases, userAssetID doesn't exist
			continue
		}

		for _, position := range principals.Items() {
			if position.Value == 0 {
				continue
			}
			jettonMaster, ok := assets[position.Key]
			if !ok {
				logger.Warn("unknown evaa asset id",
					zap.String("pool", p.id.ToRaw()),
					zap.String("asset_id", position.Key.HexString()))
				continue
			}
			asset, ok := poolAsset(provider, p, userAssetID, jettonMaster, position.Value)
			if ok {
				result = append(result, asset)
			}
		}
	}
	return result
}

func poolAsset(
	provider defi.Provider,
	pool poolInfo,
	userAssetID ton.AccountID,
	jettonMaster *ton.AccountID,
	principalValue tlb.Int64,
) (defi.Asset, bool) {
	principal := big.NewInt(int64(principalValue))
	if principal.Sign() == 0 {
		return defi.Asset{}, false
	}
	assetType := defi.AssetTypeLendingSupply
	if principal.Sign() < 0 {
		assetType = defi.AssetTypeLendingBorrow
		principal.Abs(principal)
	}

	provider.Name = pool.name
	poolID := pool.id
	assetAddress := userAssetID
	return defi.Asset{
		Type:         assetType,
		PoolAddress:  &poolID,
		AssetAddress: &assetAddress,
		Provider:     provider,
		LockedAsset:  lockedAsset(jettonMaster, *principal),
	}, true
}

func lockedAsset(jettonMaster *ton.AccountID, amount big.Int) defi.LockedAsset {
	if jettonMaster == nil {
		return defi.LockedAsset{Type: defi.LockedAssetTypeNative}
	}
	return defi.LockedAsset{
		Type:         defi.LockedAssetTypeJetton,
		JettonMaster: jettonMaster,
		Amount:       amount,
	}
}

func mustAccountIDptr(rawAddress string) *ton.AccountID {
	account := ton.MustParseAccountID(rawAddress)
	return &account
}

// assetID https://github.com/evaafi/evaa-go-sdk/blob/a9acf74075f5400eec330c7837769b075dd79aac/config/const.go#L134
func assetID(symbol string) tlb.Bits256 {
	return sha256.Sum256([]byte(symbol))
}
