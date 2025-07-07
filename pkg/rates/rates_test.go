package rates

import (
	"math"
	"reflect"
	"testing"

	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo/ton"
)

const EPS = 1e-12

func TestSortReserveAndAssetPairs(t *testing.T) {
	usdt := ton.MustParseAccountID("EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sDs")
	pTon := ton.MustParseAccountID("EQCM3B12QK1e4yZSf8GtBRT0aLMNyEsBc_DhVfRRtOEffLez")
	notcoin := ton.MustParseAccountID("EQAvlWFDxGF2lXm67y4yzC17wYKD9A0guwPkMs1gOsM__NOT")
	bolt := ton.MustParseAccountID("EQD0vdSA_NedR9uvbgN9EikRX-suesDxGeFg69XQMavfLqIw")

	assetPairs := map[ton.AccountID][]Pool{
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

	expected := map[ton.AccountID][]Pool{
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

	type Test struct {
		name  string
		pools []Pool
		// Expected output
		expectedPrice float64
	}

	notcoin := ton.MustParseAccountID("EQAvlWFDxGF2lXm67y4yzC17wYKD9A0guwPkMs1gOsM__NOT")
	bolt := ton.MustParseAccountID("EQD0vdSA_NedR9uvbgN9EikRX-suesDxGeFg69XQMavfLqIw")
	usdt := ton.MustParseAccountID("EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sDs")
	ethena := ton.MustParseAccountID("EQAIb6KmdfdDR7CN1GBqVJuP25iCnLKCvBlJ07Evuu2dzP5f")
	redo := ton.MustParseAccountID("EQBZ_cafPyDr5KUTs0aNxh0ZTDhkpEZONmLJA2SNGlLm4Cko")
	anon := ton.MustParseAccountID("EQDv-yr41_CZ2urg2gfegVfa44PDPjIK9F-MilEDKDUIhlwZ")
	switchToken := ton.MustParseAccountID("EQBxo4huVJXaf1ZOdwnnDdxa9OVoyNGhXMsJVzobmxSWITCH")
	ston := ton.MustParseAccountID("EQA2kCVNwVsil2EM2mB0SkXytxCqQjS4mttjDpnXmwG9T6bO")
	tsTon := ton.MustParseAccountID("EQC98_qAmNEptUtPc7W6xdHh_ZHrBUFpw5Ft_IzNU20QAJav")
	kton := ton.MustParseAccountID("EQA2rQ-kMzVgK2lWAtWy6Y_UVia4xv8S-_Q4Ixo3y5kzUfdx")

	tests := []Test{
		{
			name: "notcoin->ton",
			pools: []Pool{
				{Assets: []Asset{
					{Account: notcoin, Decimals: 9, Reserve: 1.8417267910171e+17, HoldersCount: 2865084},
					{Account: ton.MustParseAccountID("0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c"), Decimals: 9, Reserve: 1.25593435171918e+14, HoldersCount: 82}},
					Invariant: XYInv,
				},
			},
			expectedPrice: 0.0006819330412333231,
		},
		{
			name: "usdt->ton",
			pools: []Pool{
				{Assets: []Asset{
					{Account: usdt, Decimals: 6, Reserve: 1.2723222601996e+13, HoldersCount: 2649800},
					{Account: ton.MustParseAccountID("0:671963027f7f85659ab55b821671688601cdcf1ee674fc7fbbb1a776a18d34a3"), Decimals: 9, Reserve: 3.379910335458364e+15, HoldersCount: 39}},
					Invariant: XYInv,
				},
			},
			expectedPrice: 0.26564891939626434,
		},
		{
			name: "bolt->ton",
			pools: []Pool{
				{Assets: []Asset{
					{Account: ton.MustParseAccountID("0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c"), Decimals: 9, Reserve: 2.120750126788e+12, HoldersCount: 0},
					{Account: bolt, Decimals: 9, Reserve: 1.24636479861718e+14, HoldersCount: 22384}},
					Invariant: XYInv,
				},
			},
			expectedPrice: 0.017015484785360878,
		},
		{
			name: "ethena->ton",
			pools: []Pool{
				{Assets: []Asset{
					{Account: ethena, Decimals: 6, Reserve: 3968990293365, HoldersCount: 5169},
					{Account: ton.MustParseAccountID("0:671963027f7f85659ab55b821671688601cdcf1ee674fc7fbbb1a776a18d34a3"), Decimals: 6, Reserve: 4195613897538, HoldersCount: 4}},
					Invariant: IterativeInv,
					Amp:       100,
				},
			},
			expectedPrice: 1.0010715423976582,
		},
		{
			name: "redo->ton",
			pools: []Pool{
				{Assets: []Asset{
					{Account: redo, Decimals: 6, Reserve: 4059781684283, HoldersCount: 1317},
					{Account: ton.MustParseAccountID("0:671963027f7f85659ab55b821671688601cdcf1ee674fc7fbbb1a776a18d34a3"), Decimals: 9, Reserve: 3941626877432000, HoldersCount: 7}},
					Invariant: IterativeInv,
					Amp:       100,
				},
			},
			expectedPrice: 0.99940092953436,
		},
		{
			name: "anon->ton",
			pools: []Pool{
				{Assets: []Asset{
					{Account: anon, Decimals: 9, Reserve: 0, HoldersCount: 1000},
					{Account: ton.MustParseAccountID("0:671963027f7f85659ab55b821671688601cdcf1ee674fc7fbbb1a776a18d34a3"), Decimals: 9, Reserve: 3941626877432000, HoldersCount: 7}},
					Invariant: IterativeInv,
					Amp:       100,
				},
			},
			expectedPrice: 0,
		},
		{
			name: "switch->ton",
			pools: []Pool{
				{Assets: []Asset{
					{Account: switchToken, Decimals: 9, Reserve: 126565280078085986, HoldersCount: 1317},
					{Account: ton.MustParseAccountID("0:671963027f7f85659ab55b821671688601cdcf1ee674fc7fbbb1a776a18d34a3"), Decimals: 9, Reserve: 14187521078376, HoldersCount: 7}},
					Invariant: WXYInv,
					Weight:    0.2,
				},
			},
			expectedPrice: 0.0004483858786429528,
		},
		{
			name: "ston->ton",
			pools: []Pool{
				{Assets: []Asset{
					{Account: ston, Decimals: 9, Reserve: 57898953442455, HoldersCount: 1317},
					{Account: ton.MustParseAccountID("0:671963027f7f85659ab55b821671688601cdcf1ee674fc7fbbb1a776a18d34a3"), Decimals: 9, Reserve: 9398045691390, HoldersCount: 7}},
					Invariant: WXYInv,
					Weight:    0.25,
				},
			},
			expectedPrice: 0.4869541744341161,
		},
		{
			name: "tston->ton",
			pools: []Pool{
				{Assets: []Asset{
					{Account: ton.MustParseAccountID("0:671963027f7f85659ab55b821671688601cdcf1ee674fc7fbbb1a776a18d34a3"), Decimals: 9, Reserve: 48693539709107, HoldersCount: 7},
					{Account: tsTon, Decimals: 9, Reserve: 65206054264465, HoldersCount: 1317}},
					Invariant: WStableSwapInv,
					Amp:       50,
					Rate:      1.065,
					Weight:    0.25,
				},
			},
			expectedPrice: 1.0653657385933382,
		},
		{
			name: "kton->ton",
			pools: []Pool{
				{Assets: []Asset{
					{Account: kton, Decimals: 9, Reserve: 32841708701843, HoldersCount: 1317},
					{Account: ton.MustParseAccountID("0:671963027f7f85659ab55b821671688601cdcf1ee674fc7fbbb1a776a18d34a3"), Decimals: 9, Reserve: 5793846300860, HoldersCount: 7}},
					Invariant: WStableSwapInv,
					Amp:       50,
					Rate:      1.0210416258250017,
					Weight:    0.75,
				},
			},
			expectedPrice: 1.0653657385933382,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, pool := range test.pools {
				_, price := calculatePoolPrice(pool.Assets[0], pool.Assets[1], pools, pool.Invariant, pool.Amp, pool.Weight, pool.Rate)
				// diff between prices must be <= 1e-12 (accuracy up to 12 decimals)
				if math.Abs(price-test.expectedPrice) > math.Max(math.Max(price, test.expectedPrice)*EPS, EPS) {
					t.Errorf("Unexpected price for account: got %v, want %v\n", price, test.expectedPrice)
				}
			}
		})
	}
}

func TestParseStonFiJettonsAssets(t *testing.T) {
	tests := []struct {
		name     string
		csv      string
		expected []Pool
	}{
		{
			name: "Successful parsing of Scaleton and USD₮ jettons",
			csv: `
asset_0_account_id,asset_1_account_id,asset_0_reserve,asset_1_reserve,asset_0_metadata,asset_1_metadata,asset_0_holders,asset_1_holders,lp_jetton,total_supply,lp_jetton_decimals
0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c,0:65aac9b5e380eae928db3c8e238d9bc0d61a9320fdc2bc7a2f6c87d6fedf9208,356773586306,572083446808,"{""decimals"":""9"",""name"":""Proxy TON"",""symbol"":""pTON""}","{""name"":""Scaleton"",""symbol"":""SCALE""}",52,17245,0:4d70707f7f62d432157dd8f1a90ce7421b34bcb2ecc4390469181bc575e4739f,1000000000000000,9
0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe,0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c,54581198678395,9745288354931876,"{""decimals"":""6"",""name"":""Tether USD"",""symbol"":""USD₮""}","{""decimals"":""9"",""name"":""Proxy TON"",""symbol"":""pTON""}",1038000,52,0:4d70707f7f62d432157dd8f1a90ce7421b34bcb2ecc4390469181bc575e4739f,2000000000000000,9
`,
			expected: []Pool{
				{
					Assets: []Asset{
						{Account: ton.MustParseAccountID("0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c"), Decimals: 9, Reserve: 356773586306, HoldersCount: 52},
						{Account: ton.MustParseAccountID("0:65aac9b5e380eae928db3c8e238d9bc0d61a9320fdc2bc7a2f6c87d6fedf9208"), Decimals: 9, Reserve: 572083446808, HoldersCount: 17245},
					},
					Invariant: XYInv,
				},
				{
					Assets: []Asset{
						{Account: ton.MustParseAccountID("0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe"), Decimals: 6, Reserve: 54581198678395, HoldersCount: 1038000},
						{Account: ton.MustParseAccountID("0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c"), Decimals: 9, Reserve: 9745288354931876, HoldersCount: 52},
					},
					Invariant: XYInv,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assets, _, err := convertStonFiPoolResponse([]byte(tt.csv))
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
		expected []Pool
	}{
		{
			name: "Successful parsing of Spintria and AI Coin jettons",
			csv: `
asset_0_account_id,asset_1_account_id,asset_0_native,asset_1_native,asset_0_reserve,asset_1_reserve,asset_0_metadata,asset_1_metadata,is_stable,asset_0_holders,asset_1_holders,lp_jetton,total_supply,lp_jetton_decimals
NULL,0:022d70f08add35b2d8aa2bd16f622268d7996e5737c3e7353cbb00d2aba257c5,true,false,100171974809,1787220634679,NULL,"{""decimals"":""8"",""name"":""Spintria"",""symbol"":""SP""}",false,0,3084,0:4d70707f7f62d432157dd8f1a90ce7421b34bcb2ecc4390469181bc575e4739f,0,9
NULL,0:48cef1de34697508200b8026bf882f8e88aff894586cfd304ab513633fa7e2d3,true,false,22004762576054,4171862045823,NULL,"{""decimals"":""9"",""name"":""AI Coin"",""symbol"":""AIC""}",false,0,1239,0:4d70707f7f62d432157dd8f1a90ce7421b34bcb2ecc4390469181bc575e4739f,0,9
0:48cef1de34697508200b8026bf882f8e88aff894586cfd304ab513633fa7e2d3,0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe,false,false,468457277157,13287924673,"{""decimals"":""9"",""name"":""AI Coin"",""symbol"":""AIC""}","{""decimals"":""6"",""name"":""Tether USD"",""symbol"":""USD₮""}",false,1239,1039987,0:4d70707f7f62d432157dd8f1a90ce7421b34bcb2ecc4390469181bc575e4739f,0,9`,
			expected: []Pool{
				{
					Assets: []Asset{
						{Account: references.PTonV1, Decimals: 9, Reserve: 100171974809, HoldersCount: 0},
						{Account: ton.MustParseAccountID("0:022d70f08add35b2d8aa2bd16f622268d7996e5737c3e7353cbb00d2aba257c5"), Decimals: 8, Reserve: 1787220634679, HoldersCount: 3084},
					},
					Invariant: XYInv,
				},
				{
					Assets: []Asset{
						{Account: references.PTonV1, Decimals: 9, Reserve: 22004762576054, HoldersCount: 0},
						{Account: ton.MustParseAccountID("0:48cef1de34697508200b8026bf882f8e88aff894586cfd304ab513633fa7e2d3"), Decimals: 9, Reserve: 4171862045823, HoldersCount: 1239},
					},
					Invariant: XYInv,
				},
				{
					Assets: []Asset{
						{Account: ton.MustParseAccountID("0:48cef1de34697508200b8026bf882f8e88aff894586cfd304ab513633fa7e2d3"), Decimals: 9, Reserve: 468457277157, HoldersCount: 1239},
						{Account: ton.MustParseAccountID("0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe"), Decimals: 6, Reserve: 13287924673, HoldersCount: 1039987},
					},
					Invariant: XYInv,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assets, _, err := convertDeDustPoolResponse([]byte(tt.csv))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(assets, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, assets)
			}
		})
	}
}
