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

func (m *mockInfoSource) STONfiPools(ctx context.Context, pools []core.STONfiPoolID) (map[tongo.AccountID]core.STONfiPool, error) {
	return map[tongo.AccountID]core.STONfiPool{}, nil
}

func (m *mockInfoSource) SubscriptionInfos(ctx context.Context, ids []core.SubscriptionID) (map[tongo.AccountID]core.SubscriptionInfo, error) {
	return map[tongo.AccountID]core.SubscriptionInfo{}, nil
}

func (m *mockInfoSource) DedustPools(ctx context.Context, contracts []tongo.AccountID) (map[tongo.AccountID]core.DedustPool, error) {
	return map[tongo.AccountID]core.DedustPool{}, nil
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
			// subscriptions
			tongo.MustParseAccountID("0:280e6fcde865ebc2f2619ee00fdc8e11ce69b3e2981e9ca9f4847fd90be94d20"), // wallet V3R1 - reward address
			tongo.MustParseAccountID("0:b8f8fecda3fca32c0ca2e5791469ba087af68d178e8be19da7aca4362be50ba9"), // wallet V3R2 - withdraw to
			tongo.MustParseAccountID("0:5b92ca5f8ef8683432c9192c1a7b855f6cb08912c85d029f56faf918cfaa9649"), // wallet V4 - subscriber beneficiary
			tongo.MustParseAccountID("0:35a74ba451906124a313e5fc00382b98ec81994dd93826a045a7626b0b9be6d5"), // wallet W5 - subscriber beneficiary
			tongo.MustParseAccountID("0:1c274dc8fec45ebc9828c870b6721b90d89f3a7f864a1126a485997765665f0b"), // V2+W5 1: deploy with payment + prolong with caller + destroy by subscriber
			tongo.MustParseAccountID("0:6c91b3bb05143c79bf9b375a6abb8cfc0c06f32089e73e32d396b9f57a5ed5b4"), // V2+W5 2: deploy without payment + prolong without caller + destroy by beneficiary
			tongo.MustParseAccountID("0:e13eee2dacb5c0f522fd504ec37675a3ab9d9381b0ef18bc9b8bb96569e53f21"), // V2+W5 3: deploy with payment + cancel by expire
			tongo.MustParseAccountID("0:47afe208854e56c1e02e260442d986e848ec7275be2b2870e692bd96d25d11c4"), // V2+V4 1: deploy with payment + prolong with caller + destroy by subscriber
			tongo.MustParseAccountID("0:a2bf4c19c06a190b16eaeb15fc6fb0e002755a8f4384c97d4acb7908a4fb7580"), // V2+V4 2: deploy without payment + prolong without caller + destroy by subscriber
			tongo.MustParseAccountID("0:a98161ce093d7da470170150e2493a91199af14269c1de6293fd024f8e977b22"), // V2+V4 3: deploy without payment + destroy by beneficiary
			tongo.MustParseAccountID("0:2091c7f7e825f33980cc85a25a55afd7831f738acbe5ff92b8885533b03f3d33"), // V2+V4 4: deploy with payment + cancel by expire
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
			tongo.MustParseBlockID("(0,8000000000000000,55602899)"),
			tongo.MustParseBlockID("(0,8000000000000000,56373078)"),
			// dedust swap
			tongo.MustParseBlockID("(0,8000000000000000,38293409)"),
			// dedust swap from TON
			tongo.MustParseBlockID("(0,a000000000000000,45489132)"),
			tongo.MustParseBlockID("(0,d000000000000000,45499358)"),
			tongo.MustParseBlockID("(0,a000000000000000,45489138)"),
			tongo.MustParseBlockID("(0,6000000000000000,45501242)"),
			tongo.MustParseBlockID("(0,f000000000000000,45499384)"),
			tongo.MustParseBlockID("(0,2000000000000000,45500261)"),
			tongo.MustParseBlockID("(0,a000000000000000,45489148)"),
			// dedust swap to TON
			tongo.MustParseBlockID("(0,2000000000000000,45500987)"),
			tongo.MustParseBlockID("(0,d000000000000000,45500297)"),
			tongo.MustParseBlockID("(0,2000000000000000,45500992)"),
			tongo.MustParseBlockID("(0,9000000000000000,45489907)"),
			tongo.MustParseBlockID("(0,d000000000000000,45500305)"),
			tongo.MustParseBlockID("(0,2000000000000000,45500998)"),
			tongo.MustParseBlockID("(0,8000000000000000,59172679)"),
			tongo.MustParseBlockID("(0,8000000000000000,59172963)"),
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
			// failed dedust swap
			tongo.MustParseBlockID("(0,7000000000000000,45592983)"),
			// stonfi v2 swap simple
			tongo.MustParseBlockID("(0,6000000000000000,46034062)"),
			tongo.MustParseBlockID("(0,e000000000000000,46027828)"),
			tongo.MustParseBlockID("(0,9000000000000000,45998794)"),
			tongo.MustParseBlockID("(0,6000000000000000,46034070)"),
			tongo.MustParseBlockID("(0,6000000000000000,46034067)"),
			// stonfi v2 swap with ref
			tongo.MustParseBlockID("(0,2000000000000000,46145069)"),
			tongo.MustParseBlockID("(0,6000000000000000,46151880)"),
			tongo.MustParseBlockID("(0,2000000000000000,46145074)"),
			// ethena deposit stake request
			tongo.MustParseBlockID("(0,8000000000000000,55504556)"),
			// ethena withdraw stake request
			tongo.MustParseBlockID("(0,8000000000000000,55504824)"),
			// deposit liquidity bidask ton + bmTON
			tongo.MustParseBlockID("(0,2000000000000000,54478860)"),
			tongo.MustParseBlockID("(0,e000000000000000,54137670)"),
			tongo.MustParseBlockID("(0,6000000000000000,54491689)"),
			tongo.MustParseBlockID("(0,2000000000000000,54478869)"),
			//// deposit liquidity bidask ton + bmTON with refund
			tongo.MustParseBlockID("(0,8000000000000000,56013085)"),
			// deposit liquidity bidask usdt
			tongo.MustParseBlockID("(0,8000000000000000,55134140)"),
			// deposit liquidity bidask hydra + usdt
			tongo.MustParseBlockID("(0,8000000000000000,56246091)"),
			// bidask swap
			tongo.MustParseBlockID("(0,8000000000000000,56384438)"),
			tongo.MustParseBlockID("(0,8000000000000000,56435662)"),
			tongo.MustParseBlockID("(0,8000000000000000,56435563)"),
			tongo.MustParseBlockID("(0,8000000000000000,57372589)"),
			// mooncx swap
			tongo.MustParseBlockID("(0,8000000000000000,56398081)"),
			tongo.MustParseBlockID("(0,8000000000000000,56397942)"),
			// mooncx liquidity deposit
			tongo.MustParseBlockID("(0,2000000000000000,53995726)"),
			tongo.MustParseBlockID("(0,2000000000000000,53995729)"),
			tongo.MustParseBlockID("(0,2000000000000000,53995732)"),
			tongo.MustParseBlockID("(0,6000000000000000,54008087)"),
			tongo.MustParseBlockID("(0,6000000000000000,54008097)"),
			tongo.MustParseBlockID("(0,6000000000000000,54008103)"),
			tongo.MustParseBlockID("(0,e000000000000000,53659133)"),
			tongo.MustParseBlockID("(0,e000000000000000,53659136)"),
			tongo.MustParseBlockID("(0,e000000000000000,53659143)"),
			// tonco swap
			tongo.MustParseBlockID("(0,8000000000000000,56804640)"),
			tongo.MustParseBlockID("(0,8000000000000000,56834937)"),
			tongo.MustParseBlockID("(0,8000000000000000,56349139)"),
			// liquidity deposit stonfi
			tongo.MustParseBlockID("(0,8000000000000000,57142299)"),
			// liquidity deposit tonco
			tongo.MustParseBlockID("(0,8000000000000000,57250836)"),
			tongo.MustParseBlockID("(0,8000000000000000,57237195)"),
			// deposit/withdraw affluent
			tongo.MustParseBlockID("(0,8000000000000000,58000093)"),
			tongo.MustParseBlockID("(0,8000000000000000,58000202)"),
			tongo.MustParseBlockID("(0,8000000000000000,57971762)"),
			tongo.MustParseBlockID("(0,8000000000000000,58456395)"),
			tongo.MustParseBlockID("(0,8000000000000000,58582951)"),
			tongo.MustParseBlockID("(0,8000000000000000,58456309)"),
			// nft purchase
			tongo.MustParseBlockID("(0,8000000000000000,58460832)"),
			tongo.MustParseBlockID("(0,8000000000000000,60561386)"),
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
			name:           "nft getgems purchase v4",
			hash:           "ae633abecf6cd94c93fd1697dfbb05a8af1e0ba0ed455577c614d1466563cf92",
			filenamePrefix: "getgems-nft-purchase-v4",
			source: &mockInfoSource{
				OnNftSaleContracts: func(ctx context.Context, getGems []tongo.AccountID) (map[tongo.AccountID]core.NftSaleContract, error) {
					return map[tongo.AccountID]core.NftSaleContract{
						tongo.MustParseAccountID("0:079279ed0fd9e7e1e6b4d814c451f18cd76b91bc81478d7f8923ee8945c63be1"): {
							NftPrice: 1_200_000_000,
							Owner:    g.Pointer(tongo.MustParseAccountID("0:49486d3e13ae8b9df7f992e6e4c0c4e6685d66f1bd06ed223de26d7156011481")),
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
			name:           "dedust swap jettons",
			hash:           "831c7f1efaef9ac58fd39981468cea2bbd9c86a1bb72fc425cfc7734ae4a282f",
			filenamePrefix: "dedust-swap-jettons",
		},
		{
			name:           "dedust swap from TON",
			hash:           "05536d0c200ac0d02f93e369fe29571c13b734b7f741c9fb366786d8144f1430",
			filenamePrefix: "dedust-swap-from-ton",
		},
		{
			name:           "dedust swap to TON",
			hash:           "d2249d07091e57c43bcc6978db643dc6af14755e115ac48f6630a315b5e53498",
			filenamePrefix: "dedust-swap-to-ton",
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
				StonfiV1PTONStraw,
				JettonTransferClassicStraw,
			},
		},
		{
			name:           "megatonfi swap",
			hash:           "6a4c8e0dca5b052ab75f535df9d42ede949054f0004d3dd7aa6197af9dff0e1e",
			filenamePrefix: "megatonfi-swap",
			straws: []Merger{
				StonfiV1PTONStraw,
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
		{
			name:           "failed simple transfer",
			hash:           "63d358331c0154ade48ab92b4634c3fff004f42ce7201a37973938862d232c0f",
			filenamePrefix: "failed-simple-transfer",
		},
		{
			name:           "simple transfers, one of two failed",
			hash:           "ac0b8bf04949cb72759832ec6c123b3677b8ca140899ac859aa66a558e4f4c11",
			filenamePrefix: "simple-transfers-one-failed",
		},
		{
			name:           "failed dedust swap",
			hash:           "887c7763f41ca4a4b9de28900ab514caabc0c27ed5b41d9918d60f5e7f4a9d96",
			filenamePrefix: "failed-dedust-swap",
		},
		{
			name:           "stonfi v2 swap simple",
			hash:           "3fa256638e5f6cd356afa70eb37c89de80846973dea0c9c46adf4df5cca39a68",
			filenamePrefix: "stonfi-v2-swap-simple",
		},
		{
			name:           "stonfi v2 swap with ref",
			hash:           "d70fddb4786c04932669bf589ee73c16293115a1927dfbee5b719304232e2e1b",
			filenamePrefix: "stonfi-v2-swap-ref",
		},
		{
			name:           "subscription V2 + wallet W5 deploy with payment",
			hash:           "a9c8ffdb11f1d6f80feae77c7fcbefad48dbb95999c3524538d832c5c6a7ff6c",
			filenamePrefix: "deploy-with-payment-subscription-v2-wallet-w5",
		},
		{
			name:           "subscription V2 + wallet W5 deploy without payment",
			hash:           "043ae4fb3ef7546262a709aea01a2e4fdfb6fce66826974eeb7d9eaa61659815",
			filenamePrefix: "deploy-without-payment-subscription-v2-wallet-w5",
		},
		{
			name:           "subscription V2 + wallet W5 prolong with caller",
			hash:           "faa897623808c6d2f8eaf3effb04d1944451c08323064ab4d11aeb568a2179d7",
			filenamePrefix: "prolong-with-caller-subscription-v2-wallet-w5",
		},
		{
			name:           "subscription V2 + wallet W5 prolong without caller",
			hash:           "d877ae7ab945912a8d4d5a2759e747af133c763d3d3749619021b99bf436f794",
			filenamePrefix: "prolong-without-caller-subscription-v2-wallet-w5",
		},
		{
			name:           "subscription V2 + wallet W5 destroy by subscriber",
			hash:           "4adceddfffe48e8f58a5a4519ecbdac00b40e1e139e5a147b4a1b884cd26332d",
			filenamePrefix: "destroy-by-subscriber-subscription-v2-wallet-w5",
		},
		{
			name:           "subscription V2 + wallet W5 destroy by beneficiary",
			hash:           "fb84cdcd719debf6937720f26c354c8ff0358d978af36ea682b7a24b2581c9e2",
			filenamePrefix: "destroy-by-beneficiary-subscription-v2-wallet-w5",
		},
		{
			name:           "subscription V2 + wallet W5 cancel by expire",
			hash:           "4f3309c9e68a860a20451de261e0cd10b5c8d2308c92408814889494405fe6e5",
			filenamePrefix: "cancel-by-expire-subscription-v2-wallet-w5",
		},
		{
			name:           "subscription V2 + wallet V4 deploy with payment",
			hash:           "cd3b772e94a122e81fe34acbb59777dabcfef8a4fce60cffc205a6baa13bc4ff",
			filenamePrefix: "deploy-with-payment-subscription-v2-wallet-v4",
		},
		{
			name:           "subscription V2 + wallet V4 deploy without payment",
			hash:           "7a0150ed126ec7339af9a76337cf776b3712f69494e1cdefe601bfe7ababee58",
			filenamePrefix: "deploy-without-payment-subscription-v2-wallet-v4",
		},
		{
			name:           "subscription V2 + wallet V4 prolong with caller",
			hash:           "b2b96a30aa07124deaead600df33991e68be1e6f45dd1f273b31b571a1a4012d",
			filenamePrefix: "prolong-with-caller-subscription-v2-wallet-v4",
		},
		{
			name:           "subscription V2 + wallet V4 prolong without caller",
			hash:           "b25c2e62568777f1ae87cc92bc0764bce0b8fb125c2e6e609f49fbb04156a357",
			filenamePrefix: "prolong-without-caller-subscription-v2-wallet-v4",
		},
		{
			name:           "subscription V2 + wallet V4 destroy by subscriber",
			hash:           "41a0256861a62d2ff0146dd48349650e9447f1f6195885c25300dc58e142ac62",
			filenamePrefix: "destroy-by-subscriber-subscription-v2-wallet-v4",
		},
		{
			name:           "subscription V2 + wallet V4 destroy by beneficiary",
			hash:           "2cd867233ed7c4c95025ad34db74c4371c12adfd12f025dff786399c5ce269f4",
			filenamePrefix: "destroy-by-beneficiary-subscription-v2-wallet-v4",
		},
		{
			name:           "subscription V2 + wallet V4 cancel by expire",
			hash:           "b4ee52a17cda3a1b07f0cb3741f9545aa60385032b81e053092937fca0e7de84",
			filenamePrefix: "cancel-by-expire-subscription-v2-wallet-v4",
		},
		{
			name:           "ethena deposit stake request",
			hash:           "aebe3933b325fe74094406919b9bfe60f0daccba8d893fe6d2b6ef06c8f5804b",
			filenamePrefix: "ethena-deposit-stake-request",
		},
		{
			name:           "ethena withdraw stake request",
			hash:           "b8336722a26a86e03b986e9c8207b94a31105e75115af62649e1a95e0d4033bc",
			filenamePrefix: "ethena-withdraw-stake-request",
		},
		{
			name:           "deposit both ton + bmTON liquidity bidask",
			hash:           "bebf12180fa2e0a548ede0bc2aa9d3d4169eef4500f52fdff2e08238be33f6a4",
			filenamePrefix: "deposit-both-ton-bmton-liquidity-bidask",
		},
		{
			name:           "deposit both ton + bmTON liquidity bidask with refund",
			hash:           "0f80ada2a5f96e615c85039a92b860e1237cf956e7b440acf2a904e11f061aa8",
			filenamePrefix: "deposit-both-ton-bmton-liquidity-bidask-with-refund",
		},
		{
			name:           "deposit usdt liquidity bidask",
			hash:           "3c3981df59c333abb8d0b3aeb11c75889f4403870bb2afa9353ffde2fb2b718a",
			filenamePrefix: "deposit-usdt-liquidity-bidask",
		},
		{
			name:           "deposit hydra + usdt liquidity bidask",
			hash:           "705b8261a3ba7790220233d81726131c4ab051396ea69b97e1340d5224de65f7",
			filenamePrefix: "deposit-hydra-usdt-liquidity-bidask",
		},
		{
			name:           "bidask usdt usde swap",
			hash:           "89615121509a69920399f94aa5adfa9ac1b6b2e968ed772c7d3e6067f10530bf",
			filenamePrefix: "bidask-usdt-usde-swap",
		},
		{
			name:           "bidask usdt ton swap",
			hash:           "93d5bee270c65516b6213ace03c84110c0c8fa2b1626f21dec5b848eb88ba334",
			filenamePrefix: "bidask-usdt-ton-swap",
		},
		{
			name:           "bidask ton usdt swap",
			hash:           "f1cc8a4b6108c27df57a92ddb84fb25ed4f1f866c6b10c75905f34903f47edcf",
			filenamePrefix: "bidask-ton-usdt-swap",
		},
		{
			name:           "bidask v2 usdt ton swap",
			hash:           "8ab5cc797da6d5788ad68fc48f1ec15ee4b3a1df75c918fae2376461f2690918",
			filenamePrefix: "bidask-v2-usdt-ton-swap",
		},
		{
			name:           "mooncx usdt-ton swap",
			hash:           "1d1ce4629fc5613d68f066da2044354d963b982f25ba8be6ea3634f81cbbf88b",
			filenamePrefix: "mooncx-usdt-ton-swap",
		},
		{
			name:           "mooncx ton-usdt swap",
			hash:           "67aee84fa6df5fcdf85bcb9330c9181fd0629d686bf5b800420deef9e6850a99",
			filenamePrefix: "mooncx-ton-usdt-swap",
		},
		{
			name:           "deposit ton + usdt liquidity mooncx",
			hash:           "f73fa4e43b75d900320bcce05caf8a18a6dcef364049e5a7303466f9dcaac917",
			filenamePrefix: "deposit-ton-usdt-liquidity-mooncx",
		},
		{
			name:           "tonco ton-usdt swap",
			hash:           "780cab64043ac03070f1018f98601939a2e94681278be1c1f4908bb1779d049e",
			filenamePrefix: "tonco-ton-usdt-swap",
		},
		{
			name:           "tonco usdt-ton swap",
			hash:           "47cda8509e406d6475f173ad5a69645b2158db3191d8c087d5bbe4e8c49474ca",
			filenamePrefix: "tonco-usdt-ton-swap",
		},
		{
			name:           "tonco storm-usdt swap",
			hash:           "410e1fda05a63baf1ab3227886755dad96450922c400538910fbf91efc070488",
			filenamePrefix: "tonco-storm-usdt-swap",
		},
		{
			name:           "old withdraw stake request",
			hash:           "ba379e2e3f7636cc7a00d867d3f5213a681c0331b603226c1efb04697e9432f4",
			filenamePrefix: "old-withdraw-stake-request",
		},
		{
			name:           "new withdraw stake request",
			hash:           "444e275c0fc0e8d76f2075363ff2bdc8756f315c2e159bda0c5e40b899260e07",
			filenamePrefix: "new-withdraw-stake-request",
		},
		{
			name:           "deposit ton + storm liquidity stonfi",
			hash:           "3db0c9bfb9ad1944f55ef35ac90245b03c82b4a797726b94bfff93c84d84943b",
			filenamePrefix: "deposit-ton-storm-liquidity-stonfi",
		},
		{
			name:           "deposit ton liquidity tonco",
			hash:           "ede6bf7c4dd8c1f236a1048d5401cd8e7156f256476f639051814baf555c4fa5",
			filenamePrefix: "deposit-ton-liquidity-tonco",
		},
		{
			name:           "deposit ton + usdt liquidity tonco",
			hash:           "09c1061806e01b94fa8bf5e8fb9130f42a401f46137ce7e582cf4642d40d26da",
			filenamePrefix: "deposit-ton-usdt-liquidity-tonco",
		},
		{
			name:           "deposit usdt earn affluent",
			hash:           "30b1487a16d3b04e19b2d66ab993eaaba0e3ceb7a91bb022bd897a0d8343eb38",
			filenamePrefix: "deposit-usdt-earn-affluent",
		},
		{
			name:           "deposit usdt earn affluent with single oracle",
			hash:           "ef5c2f765be8f59fa2bec8d7a2fa6f2a5f6a7a226de4620d6b80ea4db8c714c4",
			filenamePrefix: "deposit-usdt-earn-affluent-with-single-oracle",
		},
		{
			name:           "instant withdraw usdt affluent",
			hash:           "3219afb5d13fd687fc616cc7b0ccf6de7eaa8e4510a15aceb708ad4a0df27bba",
			filenamePrefix: "instant-withdraw-usdt-affluent",
		},
		{
			name:           "instant withdraw request usdt affluent with single oracle",
			hash:           "0ed86c1e6d97887f0e644174dc8063afabfe0a088cdd1987033baf468c253b94",
			filenamePrefix: "instant-withdraw-request-usdt-affluent-with-single-oracle",
		},
		{
			name:           "instant withdraw request usdt affluent with multiple oracles",
			hash:           "42288aca8358bb322c2fa0a97d549a675b68d535ed89af3c311bdfe437e4f576",
			filenamePrefix: "instant-withdraw-request-usdt-affluent-with-multiple-oracle",
		},
		{
			name:           "withdraw request usdt affluent",
			hash:           "d171a300521c222094899c6b1b5fb15992a5eb4ba9dc7f8ac9c6033fed647d1c",
			filenamePrefix: "withdraw-request-usdt-affluent",
		},
		{
			name:           "dedust swap with omniston old",
			filenamePrefix: "dedust-swap-with-omniston-old",
			hash:           "668af03f00aaad8cfe1065151dbdda96310d4dad1c42cd86ca27a937e0741108",
		},
		{
			name:           "dedust swap with omniston new",
			filenamePrefix: "dedust-swap-with-omniston-new",
			hash:           "2e3eb4b79911185891d54c042f648d5e6764ce7569b3071582177ba84a2097f0",
		},
		{
			name:           "flawed jetton transfer",
			filenamePrefix: "flawed-jetton-transfer",
			hash:           "2aed6cf09d52919ea67a057407b9dfb3d758a30ff89385ba97ed8c76c65c5252",
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
			actionsList = EnrichWithIntentions(trace, actionsList)
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
