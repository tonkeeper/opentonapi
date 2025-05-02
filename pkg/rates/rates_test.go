package rates

import (
	"reflect"
	"testing"

	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/ton"
)

func TestSortReserveAndAssetPairs(t *testing.T) {
	usdt := ton.MustParseAccountID("EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sDs")
	pTon := ton.MustParseAccountID("EQCM3B12QK1e4yZSf8GtBRT0aLMNyEsBc_DhVfRRtOEffLez")
	notcoin := ton.MustParseAccountID("EQAvlWFDxGF2lXm67y4yzC17wYKD9A0guwPkMs1gOsM__NOT")
	bolt := ton.MustParseAccountID("EQD0vdSA_NedR9uvbgN9EikRX-suesDxGeFg69XQMavfLqIw")

	assetPairs := map[ton.AccountID][]Assets{
		notcoin: {
			{Assets: []Asset{{Account: notcoin, Reserve: 40}, {Account: usdt, Reserve: 50}}},
			{Assets: []Asset{{Account: pTon, Reserve: 90}, {Account: notcoin, Reserve: 100}}},
			{Assets: []Asset{{Account: bolt, Reserve: 1000}, {Account: notcoin, Reserve: 25}}},
		},
		bolt: {
			{Assets: []Asset{{Account: pTon, Reserve: 20}, {Account: bolt, Reserve: 30}}},
			{Assets: []Asset{{Account: bolt, Reserve: 60}, {Account: notcoin, Reserve: 50}}},
			{Assets: []Asset{{Account: usdt, Reserve: 100}, {Account: bolt, Reserve: 100}}},
		},
	}

	expected := map[ton.AccountID][]Assets{
		notcoin: {
			{Assets: []Asset{{Account: pTon, Reserve: 90}, {Account: notcoin, Reserve: 100}}},
			{Assets: []Asset{{Account: notcoin, Reserve: 40}, {Account: usdt, Reserve: 50}}},
			{Assets: []Asset{{Account: bolt, Reserve: 1000}, {Account: notcoin, Reserve: 25}}},
		},
		bolt: {
			{Assets: []Asset{{Account: usdt, Reserve: 100}, {Account: bolt, Reserve: 100}}},
			{Assets: []Asset{{Account: bolt, Reserve: 60}, {Account: notcoin, Reserve: 50}}},
			{Assets: []Asset{{Account: pTon, Reserve: 20}, {Account: bolt, Reserve: 30}}},
		},
	}

	assetPairs = sortAssetPairs(assetPairs)

	for accountID, sortedAssets := range assetPairs {
		expectedAssets := expected[accountID]
		for i, asset := range sortedAssets[0].Assets {
			if asset != expectedAssets[0].Assets[i] {
				t.Errorf("Mismatch for account %v: expected %v, got %v\n", accountID, expectedAssets, sortedAssets)
			}
		}
	}
}

func TestCalculatePoolPrice(t *testing.T) {
	pools := map[ton.AccountID]float64{
		references.PTonV1: 1,
		references.PTonV2: 1,
	}

	notcoin := ton.MustParseAccountID("EQAvlWFDxGF2lXm67y4yzC17wYKD9A0guwPkMs1gOsM__NOT")
	bolt := ton.MustParseAccountID("EQD0vdSA_NedR9uvbgN9EikRX-suesDxGeFg69XQMavfLqIw")
	usdt := ton.MustParseAccountID("EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sDs")

	assetPairs := map[ton.AccountID][]Assets{
		usdt: {
			{Assets: []Asset{
				{Account: usdt, Decimals: 6, Reserve: 1.2723222601996e+13, HoldersCount: 2649800},
				{Account: ton.MustParseAccountID("0:671963027f7f85659ab55b821671688601cdcf1ee674fc7fbbb1a776a18d34a3"), Decimals: 9, Reserve: 3.379910335458364e+15, HoldersCount: 39}},
			},
		},
		bolt: {
			{Assets: []Asset{
				{Account: ton.MustParseAccountID("0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c"), Decimals: 9, Reserve: 2.120750126788e+12, HoldersCount: 0},
				{Account: bolt, Decimals: 9, Reserve: 1.24636479861718e+14, HoldersCount: 22384}},
			},
		},
		notcoin: {
			{Assets: []Asset{
				{Account: notcoin, Decimals: 9, Reserve: 1.8417267910171e+17, HoldersCount: 2865084},
				{Account: ton.MustParseAccountID("0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c"), Decimals: 9, Reserve: 1.25593435171918e+14, HoldersCount: 82}},
			},
		},
	}

	assetPairs = sortAssetPairs(assetPairs)
	for attempt := 0; attempt < 3; attempt++ {
		for _, assets := range assetPairs {
			for _, asset := range assets {
				accountID, price := calculatePoolPrice(asset.Assets[0], asset.Assets[1], pools, asset.IsStable)
				if price == 0 {
					continue
				}
				if _, ok := pools[accountID]; !ok {
					pools[accountID] = price
					break
				}
			}
		}
	}

	expectedPrices := map[ton.AccountID]float64{
		notcoin: 0.0006819330412333231,
		bolt:    0.017015484785360878,
		usdt:    0.26564891939626434,
	}

	for accountID, expectedPrice := range expectedPrices {
		if actualPrice, ok := pools[accountID]; ok {
			if actualPrice != expectedPrice {
				t.Errorf("Unexpected price for account %v: got %v, want %v\n", accountID, actualPrice, expectedPrice)
			}
		} else {
			t.Errorf("Missing price for account %v\n", accountID)
		}
	}
}

func TestParseStonFiJettonsAssets(t *testing.T) {
	tests := []struct {
		name     string
		csv      string
		expected []Assets
	}{
		{
			name: "Successful parsing of Scaleton and USD₮ jettons",
			csv: `
asset_0_account_id,asset_1_account_id,asset_0_reserve,asset_1_reserve,asset_0_metadata,asset_1_metadata,asset_0_holders,asset_1_holders,lp_jetton,total_supply,lp_jetton_decimals
0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c,0:65aac9b5e380eae928db3c8e238d9bc0d61a9320fdc2bc7a2f6c87d6fedf9208,356773586306,572083446808,"{""decimals"":""9"",""name"":""Proxy TON"",""symbol"":""pTON""}","{""name"":""Scaleton"",""symbol"":""SCALE""}",52,17245,0:4d70707f7f62d432157dd8f1a90ce7421b34bcb2ecc4390469181bc575e4739f,1000000000000000,9
0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe,0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c,54581198678395,9745288354931876,"{""decimals"":""6"",""name"":""Tether USD"",""symbol"":""USD₮""}","{""decimals"":""9"",""name"":""Proxy TON"",""symbol"":""pTON""}",1038000,52,0:4d70707f7f62d432157dd8f1a90ce7421b34bcb2ecc4390469181bc575e4739f,2000000000000000,9
`,
			expected: []Assets{
				{
					Assets: []Asset{
						{Account: ton.MustParseAccountID("0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c"), Decimals: 9, Reserve: 356773586306, HoldersCount: 52},
						{Account: ton.MustParseAccountID("0:65aac9b5e380eae928db3c8e238d9bc0d61a9320fdc2bc7a2f6c87d6fedf9208"), Decimals: 9, Reserve: 572083446808, HoldersCount: 17245},
					},
				},
				{
					Assets: []Asset{
						{Account: ton.MustParseAccountID("0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe"), Decimals: 6, Reserve: 54581198678395, HoldersCount: 1038000},
						{Account: ton.MustParseAccountID("0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c"), Decimals: 9, Reserve: 9745288354931876, HoldersCount: 52},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assets, _, err := convertedStonFiPoolResponse([]byte(tt.csv))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(assets, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, assets)
			}
		})
	}
}

func TestParseDeDustJettonsAssets(t *testing.T) {
	tests := []struct {
		name     string
		csv      string
		expected []Assets
	}{
		{
			name: "Successful parsing of Spintria and AI Coin jettons",
			csv: `
asset_0_account_id,asset_1_account_id,asset_0_native,asset_1_native,asset_0_reserve,asset_1_reserve,asset_0_metadata,asset_1_metadata,is_stable,asset_0_holders,asset_1_holders,lp_jetton,total_supply,lp_jetton_decimals
NULL,0:022d70f08add35b2d8aa2bd16f622268d7996e5737c3e7353cbb00d2aba257c5,true,false,100171974809,1787220634679,NULL,"{""decimals"":""8"",""name"":""Spintria"",""symbol"":""SP""}",false,0,3084,0:4d70707f7f62d432157dd8f1a90ce7421b34bcb2ecc4390469181bc575e4739f,0,9
NULL,0:48cef1de34697508200b8026bf882f8e88aff894586cfd304ab513633fa7e2d3,true,false,22004762576054,4171862045823,NULL,"{""decimals"":""9"",""name"":""AI Coin"",""symbol"":""AIC""}",false,0,1239,0:4d70707f7f62d432157dd8f1a90ce7421b34bcb2ecc4390469181bc575e4739f,0,9
0:48cef1de34697508200b8026bf882f8e88aff894586cfd304ab513633fa7e2d3,0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe,false,false,468457277157,13287924673,"{""decimals"":""9"",""name"":""AI Coin"",""symbol"":""AIC""}","{""decimals"":""6"",""name"":""Tether USD"",""symbol"":""USD₮""}",false,1239,1039987,0:4d70707f7f62d432157dd8f1a90ce7421b34bcb2ecc4390469181bc575e4739f,0,9`,
			expected: []Assets{
				{
					Assets: []Asset{
						{Account: references.PTonV1, Decimals: 9, Reserve: 100171974809, HoldersCount: 0},
						{Account: ton.MustParseAccountID("0:022d70f08add35b2d8aa2bd16f622268d7996e5737c3e7353cbb00d2aba257c5"), Decimals: 8, Reserve: 1787220634679, HoldersCount: 3084},
					},
					IsStable: false,
				},
				{
					Assets: []Asset{
						{Account: references.PTonV1, Decimals: 9, Reserve: 22004762576054, HoldersCount: 0},
						{Account: ton.MustParseAccountID("0:48cef1de34697508200b8026bf882f8e88aff894586cfd304ab513633fa7e2d3"), Decimals: 9, Reserve: 4171862045823, HoldersCount: 1239},
					},
					IsStable: false,
				},
				{
					Assets: []Asset{
						{Account: ton.MustParseAccountID("0:48cef1de34697508200b8026bf882f8e88aff894586cfd304ab513633fa7e2d3"), Decimals: 9, Reserve: 468457277157, HoldersCount: 1239},
						{Account: ton.MustParseAccountID("0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe"), Decimals: 6, Reserve: 13287924673, HoldersCount: 1039987},
					},
					IsStable: false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assets, _, err := convertedDeDustPoolResponse([]byte(tt.csv))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(assets, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, assets)
			}
		})
	}
}
