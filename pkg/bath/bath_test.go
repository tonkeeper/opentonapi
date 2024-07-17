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
	"github.com/tonkeeper/tongo/liteapi"
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
	Extra    *int64 `json:"Extra,omitempty"`
}

type mockInfoSource struct {
	OnJettonMastersForWallets func(ctx context.Context, wallets []tongo.AccountID) (map[tongo.AccountID]tongo.AccountID, error)
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

func (m *mockInfoSource) STONfiPools(ctx context.Context, pools []tongo.AccountID) (map[tongo.AccountID]core.STONfiPool, error) {
	return map[tongo.AccountID]core.STONfiPool{}, nil
}

var _ core.InformationSource = &mockInfoSource{}

func TestFindActions(t *testing.T) {
	cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
	require.Nil(t, err)

	storage, err := litestorage.NewLiteStorage(zap.L(),
		cli,
		litestorage.WithPreloadAccounts([]tongo.AccountID{
			tongo.MustParseAccountID("EQAs87W4yJHlF8mt29ocA4agnMrLsOP69jC1HPyBUjJay-7l"),
			tongo.MustParseAccountID("0:54887d7c01ead183691a703afff08adc7b653fba2022df3a4963dae5171aa2ca"),
			tongo.MustParseAccountID("0:84796c47a337716be8919014070016bd16498021b27325778394ea1893544ba6"),
			tongo.MustParseAccountID("0:533f30de5722157b8471f5503b9fc5800c8d8397e79743f796b11e609adae69f"),
			tongo.MustParseAccountID("0:fe98106451d88f11b91d962dbaf032ac43134cc8f3470fb683312c258971b9ed"),
			tongo.MustParseAccountID("Ef_Mkp0G2m4bB953bRjMdbHis2gA0TlY_Tzlhd8ETtFj1CZa"),
			//failed stonfi swap
			tongo.MustParseAccountID("EQDnAzaMBU_5opYldxgC_6Y3fWH5tgnvnyv2AsaB1tRztqnx"),
			//durov
			tongo.MustParseAccountID("EQDYzZmfsrGzhObKJUw4gzdeIxEai3jAFbiGKGwxvxHinaPP"),
			//liquid withdraw
			tongo.MustParseAccountID("EQDQ0-rwRJENdk6md9-e8oApwyJJjsIgj9jJnBPQ53ytLGcs"),
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
			// failed stonfi swap
			tongo.MustParseBlockID("(0,8000000000000000,38072131)"),
			// deploy contract actions
			tongo.MustParseBlockID("(0,8000000000000000,22029126)"),
			// encrypted comment
			tongo.MustParseBlockID("(0,8000000000000000,36828763)"),
			// cancel sale at getgems
			tongo.MustParseBlockID("(0,8000000000000000,36025985)"),
			// multiple call contracts
			tongo.MustParseBlockID("(0,8000000000000000,36692636)"),
			// megatonfi swap
			tongo.MustParseBlockID("(0,8000000000000000,37707758)"),
			// deposit liquid staking
			tongo.MustParseBlockID("(0,8000000000000000,38159152)"),
			// withdraw liquid staking
			tongo.MustParseBlockID("(0,8000000000000000,38474426)"),
			// dedust swap
			tongo.MustParseBlockID("(0,8000000000000000,38293409)"),
			// wton mint
			tongo.MustParseBlockID("(0,8000000000000000,38493203)"),
			// buy nft on fragment
			tongo.MustParseBlockID("(0,8000000000000000,38499308)"),
			// liquid withdraw
			tongo.MustParseBlockID("(0,8000000000000000,38912382)"),
			// telemint deploy
			tongo.MustParseBlockID("(0,8000000000000000,38603492)"),
			// jetton transfer to another person
			tongo.MustParseBlockID("(0,8000000000000000,39685391)"),
			// ihr fee
			tongo.MustParseBlockID("(0,8000000000000000,40834551)"),
			// failed transfer with gas fee 1 TON
			tongo.MustParseBlockID("(0,a800000000000000,42491964)"),
			// jetton mint
			tongo.MustParseBlockID("(0,4000000000000000,42924030)"),
			tongo.MustParseBlockID("(0,4000000000000000,42924031)"),
			tongo.MustParseBlockID("(0,c000000000000000,42921231)"),
			tongo.MustParseBlockID("(0,4000000000000000,42924037)"),
			tongo.MustParseBlockID("(0,c000000000000000,42921237)"),
			tongo.MustParseBlockID("(0,4000000000000000,42924043)"),
			// disintar purchase
			tongo.MustParseBlockID("(0,8000000000000000,35667628)"),
			//cut jetton transfer
			tongo.MustParseBlockID("(0,8000000000000000,43480182)"),
			// jetton transfer to myself
			tongo.MustParseBlockID("(0,8000000000000000,34392947)"),
			// simple transfer
			tongo.MustParseBlockID("(0,8000000000000000,34021598)"),
			// nft transfer
			tongo.MustParseBlockID("(0,8000000000000000,33600829)"),
		}),
	)

	if err != nil {
		t.Fatal(err)
	}
	type Case struct {
		name string
		// account is used to calculate extra, if set.
		account        string
		hash           string
		filenamePrefix string
		source         core.InformationSource
		valueFlow      ValueFlow
		straws         []Merger
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
				OnNftSaleContracts: func(ctx context.Context, getGems []tongo.AccountID) (map[tongo.AccountID]core.NftSaleContract, error) {
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
			name:           "jetton transfer to myself",
			hash:           "75a0c3eef9a40479f3dd1fc82ff3728b9547a89044adb72862384c01428553bc",
			filenamePrefix: "jetton-transfer-to-myself",
		},
		{
			name:           "jetton transfer to another person",
			hash:           "e3d4dfb292db3bb612b7b9b1a0c6ae658a58a249b1152a4cd6bc1e4a60068e21",
			filenamePrefix: "jetton-transfer-to-another-person",
		},
		{
			name:           "cut jetton transfer",
			hash:           "36eabfbae72435be9bb60a0e79dfb9846509b40c4848516ca73edcfae1b97dfe",
			filenamePrefix: "cut-jetton-transfer",
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
			name:           "failed stonfi swap",
			hash:           "77f5acb88fd863e9c00a164eb551ef83c17088d1f603bf463f64f952b93406b0",
			filenamePrefix: "stonfi-failed-swap",
		},
		{
			name:           "deploy contract action",
			hash:           "657623f3db93397a3bb956e0e621d7a37e4ff27b80013a68f2dd91c8094b50e3",
			filenamePrefix: "deploy-contract",
		},
		{
			name:           "encrypted comment",
			hash:           "6f3e1f2c05df05345198a9d26456dcb51d4c78ce64ced56fb9976e92941211d3",
			filenamePrefix: "encrypted-comment",
		},
		{
			name:           "tonstake withdraw",
			hash:           "066ea6e9c4e6a6bbc0d3c2aed8a4d80a76a8d6ae47a6d1baffe1275f667dacf5",
			filenamePrefix: "tonstake-withdraw",
		},
		{
			name:           "dedust swap",
			hash:           "831c7f1efaef9ac58fd39981468cea2bbd9c86a1bb72fc425cfc7734ae4a282f",
			filenamePrefix: "dedust-swap",
		},
		{
			name:           "wton mint",
			hash:           "4b8b2a18abb6784c23eefa6f71d20aa2475c0a379dc2459c413e381fc7379803",
			filenamePrefix: "wton-mint",
		},
		{
			name:           "cancel sale at get gems ",
			hash:           "9900285c0f5a7cc18bdb61564013452461ead3bba84c6feb5195921e443cd79e",
			filenamePrefix: "getgetms-cancel-sale",
			source: &mockInfoSource{
				OnNftSaleContracts: func(ctx context.Context, getGems []tongo.AccountID) (map[tongo.AccountID]core.NftSaleContract, error) {
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
			straws: []Merger{
				NftTransferStraw,
				NftTransferNotifyStraw,
				JettonTransferPTONStraw,
				JettonTransferClassicStraw,
			},
		},
		{
			name:           "megatonfi swap",
			hash:           "6a4c8e0dca5b052ab75f535df9d42ede949054f0004d3dd7aa6197af9dff0e1e",
			filenamePrefix: "megatonfi-swap",
			straws: []Merger{
				JettonTransferPTONStraw,
				JettonTransferClassicStraw,
				MegatonFiJettonSwap,
			},
		},
		{
			name:           "deposit liquid staking",
			hash:           "482ef80fc6a147fba22c9e1d426ae0a5208e490aaccf0f0c51fd0a78b278a2a1",
			filenamePrefix: "deposit-liquid-staking",
		},
		{
			name:           "buy nft on fragment",
			hash:           "db735068c5d52ac56a35079d9821f38ba177d40adaea0073bf0464490d30ccb3",
			filenamePrefix: "buy-nft-on-fragment",
		},
		{
			name:           "liquid witdraw pending request",
			hash:           "98e8f0a2aeca64b74eecfb871f01debaf19d529d65c3b0fde9034897a79ad557",
			filenamePrefix: "liquid-withdraw-pending-request",
		},
		{
			name:           "telemint deploy",
			hash:           "08013737ecc5796d635f5c439d6d6913b4e894a8cdd86fd329c09bc51ea55239",
			filenamePrefix: "telemint-deploy",
		},
		{
			name:           "domain renew",
			hash:           "bcc48fcebc9635febcd1834ef40e1afab71c3ab46dd81ddf1d267474dae13923",
			filenamePrefix: "domain-renew",
		},
		{
			name:           "ihr fee",
			hash:           "2da9737c4da572382f7a5abfdb923f223455280089f4b627c6cb028b2b8350d2",
			filenamePrefix: "ihr-fee",
			account:        "0:6ccd325a858c379693fae2bcaab1c2906831a4e10a6c3bb44ee8b615bca1d220",
		},
		{
			name:           "failed transfer with gas fee 1 TON",
			account:        "0:a4a11a78384f92154a0c12761f2f7bc5e374f703335f5bc8f24c2e32ce4f1c26",
			hash:           "cf2eb5eb694dc3134cfb10135807efe08b4183267564c1fd04906526297e8c7f",
			filenamePrefix: "failed-transfer-with-gas-fee-1-TON",
		},
		{
			name:           "governance jetton mint",
			hash:           "e27cdf1d6987a3e74dc8d9c4a52a5b22112fe3946d0dceadf8160b74f80b9d46",
			filenamePrefix: "governance_jetton_mint",
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
				sort.Slice(jettons, func(i, j int) bool {
					return jettons[i].Address < jettons[j].Address
				})
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
			if len(c.account) > 0 {
				accountID := tongo.MustParseAccountID(c.account)
				extra := actionsList.Extra(accountID)
				results.Extra = &extra
			}
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
