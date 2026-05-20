package defi

import (
	"path"
	"strings"
	"testing"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
)

func TestJettonAssetDumpProvidersExist(t *testing.T) {
	raw := g.MustParseJson[map[tongo.AccountID]jettonAssetDumpItem](jettonAssetDump)
	for master, item := range raw {
		if _, ok := jettonDefiProviders[item.Provider]; !ok {
			t.Fatalf("master %v references unknown provider %q", master, item.Provider)
		}
	}
}

func TestJettonAssetDumpHasNoEmptyProviders(t *testing.T) {
	raw := g.MustParseJson[map[tongo.AccountID]jettonAssetDumpItem](jettonAssetDump)
	for master, item := range raw {
		if item.Provider == "" {
			t.Fatalf("master %v has empty provider", master)
		}
	}
}

func TestJettonAssetDumpHasNoSTONFiLiquidPools(t *testing.T) {
	raw := g.MustParseJson[map[tongo.AccountID]jettonAssetDumpItem](jettonAssetDump)
	for master, item := range raw {
		if item.Provider == stonfiProviderID && item.TokenType == TokenTypeLiquidPool {
			t.Fatalf("master %v should be covered by STON.fi pool rules, not dump", master)
		}
	}
}

func TestJettonDefiProvidersDump(t *testing.T) {
	for _, provider := range []string{"affluent", "ethena", "stonfi", "tonstakers"} {
		if _, ok := jettonDefiProviders[provider]; !ok {
			t.Fatalf("expected provider %q in provider dump", provider)
		}
	}
}

func TestJettonDefiProviderAssetsAreEmbedded(t *testing.T) {
	for providerID, provider := range jettonDefiProviders {
		for _, assetURL := range []string{provider.Card, provider.Full, provider.Icon} {
			if assetURL == "" {
				continue
			}
			if !strings.HasPrefix(assetURL, "/v2/assets/defi/") {
				t.Fatalf("provider %q asset URL is not local: %v", providerID, assetURL)
			}
			assetName := strings.TrimPrefix(assetURL, "/v2/assets/defi/")
			asset, err := defiAssetsFS.Open(path.Join("assets", assetName))
			if err != nil {
				t.Fatalf("provider %q asset %q is not embedded in package assets: %v", providerID, assetName, err)
			}
			_ = asset.Close()
		}
	}
}

func TestJettonAssetInfoSTONFiLiquidPoolMaster(t *testing.T) {
	master := core.JettonMaster{
		Address:  tongo.MustParseAddress("EQCGScrZe1xbyWqWDvdI6mzP-GAcAWFv6ZXuaJOuSqemxku4").ID,
		Admin:    accountIDPtr(ton.MustParseAccountID("0:92e1411ae546892f33b2c8a89ea90390d8ff4cfbb917a643b91e73f706fdb9d1")),
		CodeHash: "7GFOpKrqP3doYG8cFjKzN0094Jah58S6Q8gAnEh/7p0=",
	}

	if master.CodeHash != mustBase64Hash("ec614ea4aaea3f7768606f1c1632b3374d3de096a1e7c4ba43c8009c487fee9d") {
		t.Fatalf("unexpected code hash fixture for master %v", master.Address.ToRaw())
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

func TestSTONFiLiquidPoolUsesWhitelistedVaults(t *testing.T) {
	whitelistedAdmin := accountIDPtr(ton.MustParseAccountID("0:779dcc815138d9500e449c5291e7f12738c23d575b5310000f6a253bd607384e"))
	for _, codeHash := range []string{
		"82566ad72b6568fe7276437d3b0c911aab65ed701c13601941b2917305e81c11",
		"ec614ea4aaea3f7768606f1c1632b3374d3de096a1e7c4ba43c8009c487fee9d",
		"f04a14c3231221056c3499965e4604417e324f8e9121d840120d803288715594",
		"fbc7e8fcca72c2b9c078b359ffa936f46384491b895b6577b0a6cb3f569040bc",
		"dac47636ae899081ebd4f47dc90ef9de98456b1000591069773f683c6d601fa9",
		"cf5d0b99fa704e7cf2c9d50a8ff8b8bc7ce0b8a74e414b9c279ac544e7aade05",
	} {
		stonfiInfo, ok := stonfiPoolAssetInfo(whitelistedAdmin, mustBase64Hash(codeHash))
		if !ok {
			t.Fatalf("expected STON.fi logic asset info for whitelisted vault and code hash %v", codeHash)
		}
		assertSTONFiLiquidPoolAssetInfo(t, stonfiInfo)
	}

	notWhitelistedAdmin := accountIDPtr(ton.MustParseAccountID("0:9ecf00744c80dffb8fff2e8e0e4930844403962a747d13921403ebe34bb51b56"))
	if _, ok := stonfiPoolAssetInfo(notWhitelistedAdmin, mustBase64Hash("82566ad72b6568fe7276437d3b0c911aab65ed701c13601941b2917305e81c11")); ok {
		t.Fatalf("did not expect STON.fi logic asset info for non-whitelisted vault")
	}
}

func TestJettonAssetInfoAddressMatches(t *testing.T) {
	tests := []struct {
		name      string
		master    string
		tokenType TokenType
		tag       string
		provider  string
	}{
		{
			name:      "affluent yield token",
			master:    "0:edc50aa4808450411e615a5ad0c6224cb4bc23477ac460d5016c472f3c3c02b3",
			tokenType: TokenTypeYieldToken,
			tag:       "factorial",
			provider:  "Affluent",
		},
		{
			name:      "ethena liquid staking",
			master:    "0:d0e545323c7acb7102653c073377f7e3c67f122eb94d430a250739f109d4a57d",
			tokenType: TokenTypeLiquidStaking,
			tag:       "ethena",
			provider:  "Ethena",
		},
		{
			name:      "tonstakers liquid staking",
			master:    "0:bdf3fa8098d129b54b4f73b5bac5d1e1fd91eb054169c3916dfc8ccd536d1000",
			tokenType: TokenTypeLiquidStaking,
			tag:       "tonstakers",
			provider:  "Tonstakers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, ok := jettonAssetAddressInfos[tongo.MustParseAccountID(tt.master)]
			if !ok {
				t.Fatalf("expected address match for master %v", tt.master)
			}
			if info.TokenType != tt.tokenType {
				t.Fatalf("unexpected token type: got %q want %q", info.TokenType, tt.tokenType)
			}
			if info.DefiProvider.Tag != tt.tag {
				t.Fatalf("unexpected provider tag: got %q want %q", info.DefiProvider.Tag, tt.tag)
			}
			if info.DefiProvider.Name != tt.provider {
				t.Fatalf("unexpected provider name: got %q want %q", info.DefiProvider.Name, tt.provider)
			}
		})
	}
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

func accountIDPtr(accountID tongo.AccountID) *tongo.AccountID {
	return &accountID
}
