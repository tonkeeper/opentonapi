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
		}))
	if err != nil {
		t.Fatal(err)
	}
	type Case struct {
		name             string
		account          string
		hash             string
		filenamePrefix   string
		source           core.InformationSource
		valueFlow        ValueFlow
		additionalStraws []Straw
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
	} {
		t.Run(c.name, func(t *testing.T) {
			trace, err := storage.GetTrace(context.Background(), tongo.MustParseHash(c.hash))
			require.Nil(t, err)
			actionsList, err := FindActions(context.Background(),
				trace,
				WithStraws(append(c.additionalStraws, DefaultStraws...)),
				WithInformationSource(c.source))
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
