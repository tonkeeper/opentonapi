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
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/config"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/litestorage"
)

type nft struct {
	Address  string
	Quantity int64
}

type jetton struct {
	Address  string
	Quantity int64
}

type accountValueFlow struct {
	Account string
	Ton     int64
	Fee     int64
	Nfts    []nft
	Jettons []jetton
}

type result struct {
	Actions  []Action
	Accounts []accountValueFlow
}

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
		}))
	if err != nil {
		t.Fatal(err)
	}
	type Case struct {
		name           string
		account        string
		hash           string
		filenamePrefix string
		valueFlow      ValueFlow
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
			actionsList, err := FindActions(trace)
			require.Nil(t, err)
			results := result{
				Actions: actionsList.Actions,
			}
			for accountID, flow := range actionsList.ValueFlow.Accounts {
				var nfts []nft
				for address, quantity := range flow.Nfts {
					nfts = append(nfts, nft{Address: address.String(), Quantity: quantity})
				}
				var jettons []jetton
				for address, quantity := range flow.Jettons {
					jettons = append(jettons, jetton{Address: address.String(), Quantity: quantity.Int64()})
				}
				sort.Slice(nfts, func(i, j int) bool {
					return nfts[i].Address < nfts[j].Address
				})
				accountFlow := accountValueFlow{
					Account: accountID.String(),
					Ton:     flow.Ton,
					Fee:     flow.Fees,
					Nfts:    nfts,
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
