package rates

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tonkeeper/tongo/ton"
)

func TestCalculateJettonPriceFromStonFiPool(t *testing.T) {
	tests := []struct {
		name     string
		csv      string
		expected map[ton.AccountID]float64
	}{
		// StonFi pools operate with tokens through their pTon token (0:8cdc...4e11f7c)
		// The price of pTon to TON is known and equals 1
		// Knowing the price of pTon, we can calculate the price of the Scaleton token (0:65aa...d6fedf920) using the XY pool formula
		// We can also calculate the price of USD₮ (0:b113...c3621dfe)
		{
			name: "Successful calculate Scaleton and USD₮ jettons",
			csv: `
asset_0_account_id,asset_1_account_id,asset_0_reserve,asset_1_reserve,asset_0_metadata,asset_1_metadata,asset_0_holders,asset_1_holders
0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c,0:65aac9b5e380eae928db3c8e238d9bc0d61a9320fdc2bc7a2f6c87d6fedf9208,356773586306,572083446808,"{""decimals"":""9"",""name"":""Proxy TON"",""symbol"":""pTON""}","{""name"":""Scaleton"",""symbol"":""SCALE""}",52,17245
0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe,0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c,54581198678395,9745288354931876,"{""decimals"":""6"",""name"":""Tether USD"",""symbol"":""USD₮""}","{""decimals"":""9"",""name"":""Proxy TON"",""symbol"":""pTON""}",1038000,52
0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe,0:afc49cb8786f21c87045b19ede78fc6b46c51048513f8e9a6d44060199c1bf0c,996119000168,921942515487299500,"{""decimals"":""6"",""name"":""Tether USD"",""symbol"":""USD₮""}","{""decimals"":""9"",""name"":""Dogs"",""symbol"":""DOGS""}",1050066,881834`,
			expected: map[ton.AccountID]float64{
				ton.MustParseAccountID("0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c"): 1, // Default pTon price
				ton.MustParseAccountID("0:65aac9b5e380eae928db3c8e238d9bc0d61a9320fdc2bc7a2f6c87d6fedf9208"): 0.6236390657633181,
				ton.MustParseAccountID("0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe"): 0.17854661661707652,
				ton.MustParseAccountID("0:afc49cb8786f21c87045b19ede78fc6b46c51048513f8e9a6d44060199c1bf0c"): 0.00019291189444059384,
			},
		},
		// To display more accurate prices, the default minimum number of holders is set to 200
		// In this test, Scaleton only has 50 holders, so we do not calculate the token price
		{
			name: "Failed calculate Scaleton (insufficient holders)",
			csv: `
asset_0_account_id,asset_1_account_id,asset_0_reserve,asset_1_reserve,asset_0_metadata,asset_1_metadata,asset_0_holders,asset_1_holders
0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c,0:65aac9b5e380eae928db3c8e238d9bc0d61a9320fdc2bc7a2f6c87d6fedf9208,356773586306,572083446808,"{""decimals"":""9"",""name"":""Proxy TON"",""symbol"":""pTON""}","{""name"":""Scaleton"",""symbol"":""SCALE""}",52,10
`,
			expected: map[ton.AccountID]float64{
				// Default pTon price
				ton.MustParseAccountID("0:8cdc1d7640ad5ee326527fc1ad0514f468b30dc84b0173f0e155f451b4e11f7c"): 1,
			},
		},
	}
	var err error
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pools := make(map[ton.AccountID]float64)
			for attempt := 0; attempt < 2; attempt++ {
				respBody := io.NopCloser(bytes.NewReader([]byte(tt.csv)))
				pools, err = convertedStonFiPoolResponse(pools, respBody)
				if err != nil {
					t.Fatalf("[TestCalculateJettonPriceFromStonFiPool] failed to calculate jetton price: %v", err)
				}
			}
			if err != nil {
				t.Fatalf("[TestCalculateJettonPriceFromStonFiPool] failed to calculate jetton price: %v", err)
			}
			if !assert.Equal(t, tt.expected, pools) {
				t.Errorf("expected %v, got %v", tt.expected, pools)
			}
		})
	}
}

func TestCalculateJettonPriceFromDeDustPool(t *testing.T) {
	tests := []struct {
		name     string
		csv      string
		expected map[ton.AccountID]float64
	}{
		// In the DeDust pools, if an asset has a NULL value, it means that this asset is TON. For simplicity,
		// we treat this asset as the zero address and assign it a price of 1 relative to TON
		//
		// In this test, we are calculating the prices of Spintria tokens (0:022d70...2aba257c5) and AI Coin (0:48cef1...33fa7e2d3) relative to TON
		// We are also calculating the price of USD₮ (0:b113a9...fecdc3621dfe) relative to AI Coin (0:48cef1...33fa7e2d3)
		{
			name: "Successful calculate",
			csv: `
asset_0_account_id,asset_1_account_id,asset_0_native,asset_1_native,asset_0_reserve,asset_1_reserve,asset_0_metadata,asset_1_metadata,is_stable,asset_0_holders,asset_1_holders
NULL,0:022d70f08add35b2d8aa2bd16f622268d7996e5737c3e7353cbb00d2aba257c5,true,false,100171974809,1787220634679,NULL,"{""decimals"":""8"",""name"":""Spintria"",""symbol"":""SP""}",false,0,3084
NULL,0:48cef1de34697508200b8026bf882f8e88aff894586cfd304ab513633fa7e2d3,true,false,22004762576054,4171862045823,NULL,"{""decimals"":""9"",""name"":""AI Coin"",""symbol"":""AIC""}",false,0,1239
0:48cef1de34697508200b8026bf882f8e88aff894586cfd304ab513633fa7e2d3,0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe,false,false,468457277157,13287924673,"{""decimals"":""9"",""name"":""AI Coin"",""symbol"":""AIC""}","{""decimals"":""6"",""name"":""Tether USD"",""symbol"":""USD₮""}",false,1239,1039987
NULL,0:cf76af318c0872b58a9f1925fc29c156211782b9fb01f56760d292e56123bf87,true,false,5406255533839,3293533372962,NULL,"{""decimals"":""9"",""name"":""Hipo Staked TON"",""symbol"":""hTON""}",true,0,2181`,
			expected: map[ton.AccountID]float64{
				ton.MustParseAccountID("0:0000000000000000000000000000000000000000000000000000000000000000"): 1, // Default TON price
				ton.MustParseAccountID("0:022d70f08add35b2d8aa2bd16f622268d7996e5737c3e7353cbb00d2aba257c5"): 0.005604902543383612,
				ton.MustParseAccountID("0:48cef1de34697508200b8026bf882f8e88aff894586cfd304ab513633fa7e2d3"): 5.274566209130971,
				ton.MustParseAccountID("0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe"): 0.1859514548223248,
				ton.MustParseAccountID("0:cf76af318c0872b58a9f1925fc29c156211782b9fb01f56760d292e56123bf87"): 1.0290600202253966, // Stable pool
			},
		},
	}
	var err error
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pools := make(map[ton.AccountID]float64)
			for attempt := 0; attempt < 2; attempt++ {
				respBody := io.NopCloser(bytes.NewReader([]byte(tt.csv)))
				pools, err = convertedDeDustPoolResponse(pools, respBody)
				if err != nil {
					t.Fatalf("[TestCalculateJettonPriceFromDeDustPool] failed to calculate jetton price: %v", err)
				}
			}
			if err != nil {
				t.Fatalf("[TestCalculateJettonPriceFromDeDustPool] failed to calculate jetton price: %v", err)
			}
			if !assert.Equal(t, tt.expected, pools) {
				t.Errorf("expected %v, got %v", tt.expected, pools)
			}
		})
	}
}
