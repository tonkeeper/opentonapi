package defi

import (
	"context"
	"testing"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
)

func TestJettonAssetInfoSTONFiLiquidPoolMaster(t *testing.T) {
	master := core.JettonMaster{
		Address:  tongo.MustParseAddress("EQCGScrZe1xbyWqWDvdI6mzP-GAcAWFv6ZXuaJOuSqemxku4").ID,
		Admin:    accountIDPtr(ton.MustParseAccountID("0:92e1411ae546892f33b2c8a89ea90390d8ff4cfbb917a643b91e73f706fdb9d1")),
		CodeHash: mustBase64Hash("ec614ea4aaea3f7768606f1c1632b3374d3de096a1e7c4ba43c8009c487fee9d"),
	}

	if _, ok := jettonAssetDumpInfos[master.Address]; ok {
		t.Fatalf("did not expect STON.fi liquid pool in dump for master %v", master.Address.ToRaw())
	}

	stonfiInfo, ok := stonfiPoolAssetInfo(master.Admin, master.CodeHash)
	if !ok {
		t.Fatalf("expected STON.fi logic asset info for master %v", master.Address.ToRaw())
	}
	assertSTONFiLiquidPoolAssetInfo(t, stonfiInfo)
}

func TestJettonAssetInfoDeDustLiquidPoolMaster(t *testing.T) {
	master := core.JettonMaster{
		Address:  tongo.MustParseAddress("0:00576440c4a6f443af2fcef7b27a2277eb0c552e8b898190cce9c3393d17c6e5").ID,
		CodeHash: mustBase64Hash("778f0d3fe6482c50888970df5e787f40f3a4ab282170c035a5920877058c99d3"),
	}

	if _, ok := jettonAssetDumpInfos[master.Address]; ok {
		t.Fatalf("did not expect DeDust liquid pool in dump for master %v", master.Address.ToRaw())
	}

	infos := AssetInfos(context.Background(), staticJettonMasterSource{masters: []core.JettonMaster{master}}, zap.NewNop(), []tongo.AccountID{master.Address})
	dedustInfo, ok := infos[master.Address]
	if !ok {
		t.Fatalf("expected DeDust logic asset info for master %v", master.Address.ToRaw())
	}
	assertDeDustLiquidPoolAssetInfo(t, dedustInfo)
}

func assertSTONFiLiquidPoolAssetInfo(t *testing.T, info AssetInfo) {
	t.Helper()
	if info.TokenType != TokenTypeLiquidPool {
		t.Fatalf("unexpected token type: got %q", info.TokenType)
	}
	if info.DefiProvider.Tag != stonfiProviderID {
		t.Fatalf("unexpected provider tag: got %q", info.DefiProvider.Tag)
	}
	if info.DefiProvider.Name != "STON.fi" {
		t.Fatalf("unexpected provider name: got %q", info.DefiProvider.Name)
	}
}

func assertDeDustLiquidPoolAssetInfo(t *testing.T, info AssetInfo) {
	t.Helper()
	if info.TokenType != TokenTypeLiquidPool {
		t.Fatalf("unexpected token type: got %q", info.TokenType)
	}
	if info.DefiProvider.Tag != dedustProviderID {
		t.Fatalf("unexpected provider tag: got %q", info.DefiProvider.Tag)
	}
	if info.DefiProvider.Name != "DeDust.io" {
		t.Fatalf("unexpected provider name: got %q", info.DefiProvider.Name)
	}
}

func accountIDPtr(accountID tongo.AccountID) *tongo.AccountID {
	return &accountID
}

type staticJettonMasterSource struct {
	masters []core.JettonMaster
}

func (s staticJettonMasterSource) GetJettonMastersByAddresses(_ context.Context, _ []ton.AccountID) ([]core.JettonMaster, error) {
	return s.masters, nil
}
