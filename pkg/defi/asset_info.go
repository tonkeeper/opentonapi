package defi

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/hex"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
)

type Provider struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Link        string `json:"link"`
	MiniappLink string `json:"miniapp_link"`
	Icon        string `json:"icon"`
	Card        string `json:"card"`
	Full        string `json:"full"`
	Tag         string `json:"tag"`
}

type AssetInfo struct {
	TokenType    AssetType
	DefiProvider Provider
}

type jettonAssetDumpItem struct {
	TokenType AssetType `json:"token_type"`
	Provider  string    `json:"provider"`
}

const (
	stonfiProviderID = "stonfi"
	dedustProviderID = "dedust"
)

type JettonMasterSource interface {
	GetJettonMastersByAddresses(ctx context.Context, addresses []ton.AccountID) ([]core.JettonMaster, error)
}

//go:embed defi_masters_mapping.json
var jettonAssetDump []byte

//go:embed defi_providers.json
var jettonProvidersDump []byte

var jettonAssetDumpInfos map[tongo.AccountID]AssetInfo
var jettonDefiProviders map[string]Provider

func init() {
	jettonDefiProviders = g.MustParseJson[map[string]Provider](jettonProvidersDump)
	jettonAssetDumpInfos = parseJettonAssetDump(jettonAssetDump, jettonDefiProviders)
}

var stonfiPoolCodeHashes = map[string]bool{
	// STON.fi v1 pool, e.g. https://app.ston.fi/pools/0:1b011c80e68e5942aab7e5c79b7b4faacd4999ecda9579df58b3edfbcca414f4
	mustBase64Hash("82566ad72b6568fe7276437d3b0c911aab65ed701c13601941b2917305e81c11"): true,
	// STON.fi v2 constant product pool, e.g. https://app.ston.fi/pools/0:8649cad97b5c5bc96a960ef748ea6ccff8601c01616fe995ee6893ae4aa7a6c6
	mustBase64Hash("ec614ea4aaea3f7768606f1c1632b3374d3de096a1e7c4ba43c8009c487fee9d"): true,
	// STON.fi v2 stableswap pool, e.g. https://app.ston.fi/pools/0:022cdbe42ffd83d97056fd72aa1b26cf46479be544599edb0423abc5c737070b
	mustBase64Hash("f04a14c3231221056c3499965e4604417e324f8e9121d840120d803288715594"): true,
	// STON.fi v2 stableswap pool, e.g. https://app.ston.fi/pools/0:52518e14586245029b342b01d8e2da4e1aacd4e350d9872ad3f0ebb20165ad9d
	mustBase64Hash("fbc7e8fcca72c2b9c078b359ffa936f46384491b895b6577b0a6cb3f569040bc"): true,
	// STON.fi v2 weighted stableswap pool, e.g. https://app.ston.fi/pools/0:05ea635b2a168cadfca174d72b12744a5b57d70378e6912e8a33b6b39bd3ee9d
	mustBase64Hash("dac47636ae899081ebd4f47dc90ef9de98456b1000591069773f683c6d601fa9"): true,
	// STON.fi v2 constant product pool, e.g. https://app.ston.fi/pools/0:ad28d3fb6e911349282352604ab89ad6f3272f9aa2dc73276588f338eec3d823
	mustBase64Hash("cf5d0b99fa704e7cf2c9d50a8ff8b8bc7ce0b8a74e414b9c279ac544e7aade05"): true,
}

var dedustPoolCodeHashes = map[string]bool{
	// DeDust pool, e.g. https://dedust.io/pools/0:00576440c4a6f443af2fcef7b27a2277eb0c552e8b898190cce9c3393d17c6e5
	mustBase64Hash("778f0d3fe6482c50888970df5e787f40f3a4ab282170c035a5920877058c99d3"): true,
	// DeDust pool, e.g. https://dedust.io/pools/0:011e4c677529eaac180c20347bdcb0741410c33340bf529dfd10bd519f76a8ff
	mustBase64Hash("1275095b6da3911292406f4f4386f9e780099b854c6dee9ee2895ddce70927c1"): true,
	// DeDust pool, e.g. https://dedust.io/pools/0:0706d6f329feb86a9876c7d1dcbb189fb99482f5794f0c746c75d204b15fccac
	mustBase64Hash("c0f9d14fbc8e14f0d72cba2214165eee35836ab174130912baf9dbfa43ead562"): true,
}

func parseJettonAssetDump(data []byte, providers map[string]Provider) map[tongo.AccountID]AssetInfo {
	raw := g.MustParseJson[map[tongo.AccountID]jettonAssetDumpItem](data)
	return g.MapMapValues(raw, func(item jettonAssetDumpItem) AssetInfo {
		provider, ok := providers[item.Provider]
		if !ok {
			panic("unknown defi provider: " + item.Provider)
		}
		if !validAssetType(item.TokenType) {
			panic("unknown defi token type: " + string(item.TokenType))
		}
		return AssetInfo{
			TokenType:    item.TokenType,
			DefiProvider: provider,
		}
	})
}

func validAssetType(tokenType AssetType) bool {
	switch tokenType {
	case AssetTypeLiquidStaking, AssetTypeLiquidPool, AssetTypeYieldToken, AssetTypeLendingSupply, AssetTypeLendingBorrow:
		return true
	default:
		return false
	}
}

func stonfiPoolAssetInfo(admin *tongo.AccountID, codeHash string) (AssetInfo, bool) {
	if admin == nil {
		return AssetInfo{}, false
	}
	if _, ok := references.StonfiWhitelistVaults[*admin]; !ok {
		return AssetInfo{}, false
	}
	if !stonfiPoolCodeHashes[codeHash] {
		return AssetInfo{}, false
	}
	provider, ok := jettonDefiProviders[stonfiProviderID]
	if !ok {
		return AssetInfo{}, false
	}
	return AssetInfo{
		TokenType:    AssetTypeLiquidPool,
		DefiProvider: provider,
	}, true
}

func dedustPoolAssetInfo(codeHash string) (AssetInfo, bool) {
	if !dedustPoolCodeHashes[codeHash] {
		return AssetInfo{}, false
	}
	provider, ok := jettonDefiProviders[dedustProviderID]
	if !ok {
		return AssetInfo{}, false
	}
	return AssetInfo{
		TokenType:    AssetTypeLiquidPool,
		DefiProvider: provider,
	}, true
}

func GetProvider(tag string) (Provider, bool) {
	p, ok := jettonDefiProviders[tag]
	return p, ok
}

func mustBase64Hash(hexHash string) string {
	b, err := hex.DecodeString(hexHash)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func AssetInfos(ctx context.Context, source JettonMasterSource, logger *zap.Logger, masters []tongo.AccountID) map[tongo.AccountID]AssetInfo {
	if len(masters) == 0 {
		return nil
	}

	result := make(map[tongo.AccountID]AssetInfo)
	unresolved := make([]tongo.AccountID, 0, len(masters))
	for _, master := range masters {
		if info, ok := jettonAssetDumpInfos[master]; ok {
			result[master] = info
		} else {
			unresolved = append(unresolved, master)
		}
	}
	if len(unresolved) == 0 {
		return result
	}

	masterMetas, err := source.GetJettonMastersByAddresses(ctx, unresolved)
	if err != nil {
		logger.Warn("failed to get jetton masters for asset info", zap.Error(err))
		return result
	}
	for _, master := range masterMetas {
		stonfiInfo, stonfiOk := stonfiPoolAssetInfo(master.Admin, master.CodeHash)
		if stonfiOk {
			result[master.Address] = stonfiInfo
			continue
		}
		dedustInfo, dedustOk := dedustPoolAssetInfo(master.CodeHash)
		if dedustOk {
			result[master.Address] = dedustInfo
		}
	}
	return result
}
