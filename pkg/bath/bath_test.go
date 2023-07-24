package bath

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/config"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/litestorage"
)

type jettonItem struct {
	Address  string
	Quantity int64
}

type accountValueFlow struct {
	Account string
	Ton     int64
	Fee     int64
	Jettons []jettonItem
}

type result struct {
	Actions  []Action
	Accounts []accountValueFlow
}

type mockInfoSource struct {
	OnJettonMastersForWallets func(ctx context.Context, wallets []tongo.AccountID) (map[tongo.AccountID]tongo.AccountID, error)
	OnGetGemsContracts        func(ctx context.Context, getGems []tongo.AccountID) (map[tongo.AccountID]core.NftSaleContract, error)
	OnNftSaleContracts        func(ctx context.Context, contracts []tongo.AccountID) (map[tongo.AccountID]core.NftSaleContract, error)
}

func (m *mockInfoSource) NftSaleContracts(ctx context.Context, contracts []tongo.AccountID) (map[tongo.AccountID]core.NftSaleContract, error) {
	if m.OnNftSaleContracts == nil {
		return map[tongo.AccountID]core.NftSaleContract{}, nil
	}
	return m.OnNftSaleContracts(ctx, contracts)

}

func (m *mockInfoSource) JettonMastersForWallets(ctx context.Context, wallets []tongo.AccountID) (map[tongo.AccountID]tongo.AccountID, error) {
	if m.OnJettonMastersForWallets == nil {
		return map[tongo.AccountID]tongo.AccountID{}, nil
	}
	return m.OnJettonMastersForWallets(ctx, wallets)
}

func (m *mockInfoSource) GetGemsContracts(ctx context.Context, getGems []tongo.AccountID) (map[tongo.AccountID]core.NftSaleContract, error) {
	if m.OnGetGemsContracts == nil {
		return map[tongo.AccountID]core.NftSaleContract{}, nil
	}
	return m.OnGetGemsContracts(ctx, getGems)
}

func (m *mockInfoSource) STONfiPools(ctx context.Context, pools []tongo.AccountID) (map[tongo.AccountID]core.STONfiPool, error) {
	return map[tongo.AccountID]core.STONfiPool{}, nil
}

var _ core.InformationSource = &mockInfoSource{}

func TestFindActions(t *testing.T) {
	var servers []config.LiteServer
	if env, ok := os.LookupEnv("LITE_SERVERS"); ok {
		var err error
		servers, err = config.ParseLiteServersEnvVar(env)
		if err != nil {
			t.Fatal(err)
		}
	}

	storage, err := litestorage.NewLiteStorage(zap.L(),
		litestorage.WithLiteServers(servers),
		litestorage.WithPreloadAccounts([]tongo.AccountID{
			tongo.MustParseAccountID("EQAs87W4yJHlF8mt29ocA4agnMrLsOP69jC1HPyBUjJay-7l"),
			tongo.MustParseAccountID("0:54887d7c01ead183691a703afff08adc7b653fba2022df3a4963dae5171aa2ca"),
			tongo.MustParseAccountID("0:84796c47a337716be8919014070016bd16498021b27325778394ea1893544ba6"),
			tongo.MustParseAccountID("0:533f30de5722157b8471f5503b9fc5800c8d8397e79743f796b11e609adae69f"),
			tongo.MustParseAccountID("0:fe98106451d88f11b91d962dbaf032ac43134cc8f3470fb683312c258971b9ed"),
			tongo.MustParseAccountID("Ef_Mkp0G2m4bB953bRjMdbHis2gA0TlY_Tzlhd8ETtFj1CZa"),
		}),
		litestorage.WithPreloadBlocks([]tongo.BlockID{
			// tf nominator deposit
			tongo.MustParseBlockID("(0,8000000000000000,35205653)"),
			tongo.MustParseBlockID("(-1,8000000000000000,29537038)"),
			// tf nominator process withdraws
			tongo.MustParseBlockID("(-1,8000000000000000,28086143)"),
			tongo.MustParseBlockID("(0,8000000000000000,33674077)"),
			// tf nominator withdraw request
			tongo.MustParseBlockID("(0,8000000000000000,35988956)"),
			tongo.MustParseBlockID("(-1,8000000000000000,30311440)"),
			tongo.MustParseBlockID("(0,8000000000000000,35988959)"),
			// tf update validator set
			tongo.MustParseBlockID("(-1,8000000000000000,30311911)"),
			// stonfi swap
			tongo.MustParseBlockID("(0,8000000000000000,36716516)"),
			// stonfi swap
			tongo.MustParseBlockID("(0,8000000000000000,36693371)"),
			// deploy contract actions
			tongo.MustParseBlockID("(0,8000000000000000,22029126)"),
			// deploy contract failed
			tongo.MustParseBlockID("(0,8000000000000000,32606229)"),
			tongo.MustParseBlockID("(-1,8000000000000000,27037607)"),
			tongo.MustParseBlockID("(0,8000000000000000,32606232)"),
			// encrypted comment
			tongo.MustParseBlockID("(0,8000000000000000,36828763)"),
			// cancel sale at getgems
			tongo.MustParseBlockID("(0,8000000000000000,36025985)"),
			// multiple call contracts
			tongo.MustParseBlockID("(0,8000000000000000,36692636)"),
		}),
	)

	if err != nil {
		t.Fatal(err)
	}
	type Case struct {
		name           string
		account        string
		hash           string
		filenamePrefix string
		source         core.InformationSource
		valueFlow      ValueFlow
		straws         []Straw
	}
	for _, c := range []Case{
		{
			name:           "simple transfer",
			filenamePrefix: "simple-transfer",
			hash:           "4a419223a45d331f1e6b48adb6dbde7f498072ac5cbea527beaa090b104ac431",
		},
		{
			name:           "nft transfer",
			hash:           "648b9fb6f0781778b5128efcffd306545695e019795ca35e4a7ff981c544f0ea",
			filenamePrefix: "nft-transfer",
		},
		{
			name:           "nft purchase",
			hash:           "8feb00edd889f8a36fb8af5b4d5370190fcbe872088cd1247c445e3c3b39a795",
			filenamePrefix: "getgems-nft-purchase",
			source: &mockInfoSource{
				OnGetGemsContracts: func(ctx context.Context, getGems []tongo.AccountID) (map[tongo.AccountID]core.NftSaleContract, error) {
					return map[tongo.AccountID]core.NftSaleContract{
						tongo.MustParseAccountID("0:4495a1921ab497b0eacee0d78838f8aeaba12481c29ec592c7fd5cbdd5b5ad0e"): {
							NftPrice: 1_200_000_000,
							Owner:    g.Pointer(tongo.MustParseAccountID("0:353d4cb749da429dbee9387ff7f65c0d99a922396a3d239eff8be465422771a0")),
						},
					}, nil
				},
			},
		},
		{
			name:           "nft purchase at Disintar Marketplace",
			hash:           "7592e2c406c4320c408341989d42fae6dc18654af0f2110d76b3c44c2b0b5495",
			filenamePrefix: "disintar-nft-purchase",
			source: &mockInfoSource{
				OnNftSaleContracts: func(ctx context.Context, getGems []tongo.AccountID) (map[tongo.AccountID]core.NftSaleContract, error) {
					return map[tongo.AccountID]core.NftSaleContract{
						tongo.MustParseAccountID("0:82e5172540cc7b9aeb8dfa6a82c2053f7008f484f94472efa369a9628a8c896a"): {
							NftPrice: 1_200_000_000,
							Owner:    g.Pointer(tongo.MustParseAccountID("0:bbf8ca0967809097d1aa67afee9cb2db2b9aa5e5f66c50a5c92033dc57b1cc23")),
						},
					}, nil
				},
			},
		},
		{
			name:           "subscription initialization",
			hash:           "039265f4baeece69168d724ecfed546267d95278a7a6d6445912fe6cc1766056",
			filenamePrefix: "subscription-init",
		},
		{
			name:           "subscription prolongation",
			hash:           "d5cf39c85392e40a7f5b0e706c4df56ad89cb214e4c5b5206fbe82c6d71a09cf",
			filenamePrefix: "subscription-prolongation",
		},
		{
			name:           "jetton transfer",
			hash:           "75a0c3eef9a40479f3dd1fc82ff3728b9547a89044adb72862384c01428553bc",
			filenamePrefix: "jetton-transfer",
		},
		{
			name:           "tf nominator deposit",
			hash:           "6e90e927c76d2eeed4e854586ea788ae6e7df50b72ca422fea2c396dd03b4c2f",
			filenamePrefix: "tf-nominator-deposit",
		},
		{
			name:           "tf nominator process withdraw requests",
			hash:           "89eb69765b3b4cd2635657d60a0e6c8be9422095e0dff4b98469f40f8d0a5566",
			filenamePrefix: "tf-nominator-process-withdraw-requests",
		},
		{
			name:           "tf nominator withdraw request",
			hash:           "67247836603b8e2b538520ad57661ad594dc6f4cf2740c0c3a3529a11ae14c23",
			filenamePrefix: "tf-nominator-request-a-withdraw",
		},
		{
			name:           "tf update validator set",
			hash:           "8ee4410c8159287702c78a32e166a8566036c752092d1d8cc520e890e6181042",
			filenamePrefix: "tf-update-validator-set",
		},
		{
			name:           "stonfi swap",
			hash:           "ace68c0da7833cb042ce6049cfac5fcf5fa9f3c93bfc9c02871381253d9e2157",
			filenamePrefix: "stonfi-swap-jUSDT-STON",
		},
		{
			name:           "stonfi buying jUSDT",
			hash:           "449aae1c0b5ebe55bc7c9efa6e511bd31b659b4bc92f3f40f50598fbbd9ca243",
			filenamePrefix: "stonfi-purchase-jUSDT",
		},
		{
			name:           "deploy contract action",
			hash:           "657623f3db93397a3bb956e0e621d7a37e4ff27b80013a68f2dd91c8094b50e3",
			filenamePrefix: "deploy-contract",
		},
		{
			name:           "deploy contract failed",
			hash:           "32467cfe9b9faecf6cd2a8055bfc7243e289fec6b2f479df2bcbf78eac9826a0",
			filenamePrefix: "deploy-contract-failed",
		},
		{
			name:           "encrypted comment",
			hash:           "6f3e1f2c05df05345198a9d26456dcb51d4c78ce64ced56fb9976e92941211d3",
			filenamePrefix: "encrypted-comment",
		},
		{
			name:           "cancel sale at get gems ",
			hash:           "9900285c0f5a7cc18bdb61564013452461ead3bba84c6feb5195921e443cd79e",
			filenamePrefix: "getgetms-cancel-sale",
			source: &mockInfoSource{
				OnGetGemsContracts: func(ctx context.Context, getGems []tongo.AccountID) (map[tongo.AccountID]core.NftSaleContract, error) {
					return map[tongo.AccountID]core.NftSaleContract{
						tongo.MustParseAccountID("0:30635bdaa6736ad1ed89146b5a34e2cc21e9eca51031a463fe4c89a537304547"): {
							NftPrice: 1_200_000_000_000,
							Owner:    g.Pointer(tongo.MustParseAccountID("0:bbf8ca0967809097d1aa67afee9cb2db2b9aa5e5f66c50a5c92033dc57b1cc23")),
						},
					}, nil
				},
			},
		},
		{
			name:           "multiple call contracts",
			hash:           "e87ec0ae9ebdba400b82887462dd0908a954fe2165c1a89775742d85a5e2a5f8",
			filenamePrefix: "multiple-call-contracts",
			straws: []Straw{
				FindNFTTransfer,
				FindJettonTransfer,
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			trace, err := storage.GetTrace(context.Background(), tongo.MustParseHash(c.hash))
			require.Nil(t, err)
			source := c.source
			if c.source == nil {
				source = storage
			}
			straws := DefaultStraws
			if len(c.straws) > 0 {
				straws = c.straws
			}
			actionsList, err := FindActions(context.Background(),
				trace,
				WithStraws(straws),
				WithInformationSource(source))
			require.Nil(t, err)
			results := result{
				Actions: actionsList.Actions,
			}
			for accountID, flow := range actionsList.ValueFlow.Accounts {
				var jettons []jettonItem
				for address, quantity := range flow.Jettons {
					jettons = append(jettons, jettonItem{Address: address.String(), Quantity: quantity.Int64()})
				}
				accountFlow := accountValueFlow{
					Account: accountID.String(),
					Ton:     flow.Ton,
					Fee:     flow.Fees,
					Jettons: jettons,
				}
				results.Accounts = append(results.Accounts, accountFlow)
			}
			sort.Slice(results.Accounts, func(i, j int) bool {
				return results.Accounts[i].Account < results.Accounts[j].Account
			})
			outputFilename := fmt.Sprintf("testdata/%v.output.json", c.filenamePrefix)
			bs, err := json.MarshalIndent(results, " ", "  ")
			require.Nil(t, err)
			err = os.WriteFile(outputFilename, bs, 0644)
			require.Nil(t, err)
			inputFilename := fmt.Sprintf("testdata/%v.json", c.filenamePrefix)
			expected, err := os.ReadFile(inputFilename)
			require.Nil(t, err)

			if !bytes.Equal(bs, expected) {
				t.Fatalf("got different results, compare %v and %v", inputFilename, outputFilename)
			}
		})
	}
}
