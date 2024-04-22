package api

import (
	"context"
	"testing"

	"github.com/tonkeeper/opentonapi/pkg/chainstate"
	pkgTesting "github.com/tonkeeper/opentonapi/pkg/testing"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteapi"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/config"
	"github.com/tonkeeper/opentonapi/pkg/litestorage"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func TestHandler_GetRawAccount(t *testing.T) {
	tests := []struct {
		name           string
		params         oas.GetBlockchainRawAccountParams
		wantStatus     oas.AccountStatus
		wantAddress    string
		wantFrozenHash string
		wantLibraries  []oas.BlockchainRawAccountLibrariesItem
	}{
		{
			name:          "all good",
			params:        oas.GetBlockchainRawAccountParams{AccountID: "EQDendoireMDFMufOUzkqNpFIay83GnjV2tgGMbA64wA3siV"},
			wantAddress:   "0:de9dda22ade30314cb9f394ce4a8da4521acbcdc69e3576b6018c6c0eb8c00de",
			wantStatus:    "active",
			wantLibraries: nil,
		},
		{
			name:        "with libraries",
			params:      oas.GetBlockchainRawAccountParams{AccountID: "EQAYB7NnOFOm2X6bewRAv2zbbWBzdV13HXpgrrg4aH3M44Ei"},
			wantAddress: "0:1807b3673853a6d97e9b7b0440bf6cdb6d6073755d771d7a60aeb838687dcce3",
			wantStatus:  "active",
			wantLibraries: []oas.BlockchainRawAccountLibrariesItem{
				{
					Public: true,
					Root:   "te6ccgECCwEAAnQAART/APSkE/S88sgLAQIC0QIDAgEgBAUCASAICQHLDPQ0wMBcbAjxwCxkl8D4PpAMPht8AH4S/LiwwHTHyGCEExWtrW6jjsyghA3nvU7uo4u+E34SscF8uK++kAwcHMhgBjIywX4Rc8WIfoCy2rLH8s/Ac8WyYBA+wBw+GvwApEw4uMNgBgB1O1E0NP/Afhh0x8B+GL6AAH4Y9MfAfhk+kAB+GXTHwH4ZvQEAfhn0x8B+GjTHwH4afpAAfhq0gAw+GuAB/DH4I/hIvvgj+Em7sPLiwNMPgwjXGCD5AfhBQTD5EPLivdM/Afhs+kAw+E0hxwXy4r76RAHAAPLiwvhCUiC8lDH4QgHe+EdSEIMH9A5voXABlDDTHzCRMeL4RiGhUjC8ljL4RiKhAt4iwgDy4sGCEAcnDgAjgQD5oIEA+qkEqAcA7vhDggr68ICgUkCooFNAvvLiv/hCJKH4YlEToMjLH8nQ+EdBMIMH9Bb4ZyHwA1MgoYIJMS0AvI4WcIAYyMsF+E3PFlBCofoCEstqyXD7AJIwMeL4Q1IQqMIAjhdwgBjIywX4Ss8W+EMTqBL6AstqyXH7AJEw4vACAGE+Ev4SfhI+Ef4RvhE+EL4QcjL/8sf+EP6Assf+EXPFssf9ADLH8sf+ErPFsoAye1UgAQ8kyDCAIroMIAoA8G0hgQD6tgggjkPI+ERwAZ16qQymMAIQI6QBIMAA5jCSywfki1Lmpzb26M8Wycj4Tc8WzMnIggr68ID6AszJ0PhEVQKAQPQW+ESk+GQB5PhMcnCAGMjLBfhFzxaCC5OHAAWkFagU+gITy2oSyx/LP8zJcfsAgQD6oQ==",
				},
			},
		},
		{
			name:           "frozen",
			params:         oas.GetBlockchainRawAccountParams{AccountID: "Ef9ngXWkQ2fOYWOagoVqqDNYReae-J2-fRysgjsFzZkXa3aW"},
			wantAddress:    "-1:678175a44367ce61639a82856aa8335845e69ef89dbe7d1cac823b05cd99176b",
			wantStatus:     "frozen",
			wantFrozenHash: "599ef5e8df99ae2fae1b23a292a091bb2cb489245bca2d50153928837d98dc49",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
			require.Nil(t, err)
			liteStorage, err := litestorage.NewLiteStorage(logger, cli)
			require.Nil(t, err)
			h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage))
			require.Nil(t, err)
			account, err := h.GetBlockchainRawAccount(context.Background(), tt.params)
			require.Nil(t, err)

			require.Equal(t, tt.wantAddress, account.Address)
			require.Equal(t, tt.wantStatus, account.Status)
			require.Equal(t, tt.wantLibraries, account.Libraries)
			require.Equal(t, tt.wantFrozenHash, account.FrozenHash.Value)
		})
	}
}

func TestHandler_GetAccount(t *testing.T) {
	tests := []struct {
		name        string
		params      oas.GetAccountParams
		wantStatus  oas.AccountStatus
		wantAddress string
	}{
		{
			params:      oas.GetAccountParams{AccountID: "EQDendoireMDFMufOUzkqNpFIay83GnjV2tgGMbA64wA3siV"},
			wantAddress: "0:de9dda22ade30314cb9f394ce4a8da4521acbcdc69e3576b6018c6c0eb8c00de",
			wantStatus:  "active",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
			require.Nil(t, err)
			liteStorage, err := litestorage.NewLiteStorage(logger, cli)
			require.Nil(t, err)
			h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage))
			require.Nil(t, err)
			accountRes, err := h.GetAccount(context.Background(), tt.params)
			require.Nil(t, err)
			require.Equal(t, tt.wantAddress, accountRes.Address)
			require.Equal(t, tt.wantStatus, accountRes.Status)
		})
	}
}

func TestHandler_GetAccounts(t *testing.T) {
	tests := []struct {
		name                string
		params              oas.OptGetAccountsReq
		wantStatuses        map[string]string
		wantNames           map[string]string
		wantBadRequestError string
	}{
		{
			params: oas.OptGetAccountsReq{
				Value: oas.GetAccountsReq{
					AccountIds: []string{
						"-1:3333333333333333333333333333333333333333333333333333333333333333",
						"-1:5555555555555555555555555555555555555555555555555555555555555555",
						"0:a3935861f79daf59a13d6d182e1640210c02f98e3df18fda74b8f5ab141abf18",
						"0:a3935861f79daf59a13d6d182e1640210c02f98e3df18fda74b8f5ab141abf17",
					},
				},
			},
			wantStatuses: map[string]string{
				"-1:3333333333333333333333333333333333333333333333333333333333333333": "active",
				"-1:5555555555555555555555555555555555555555555555555555555555555555": "active",
				"0:a3935861f79daf59a13d6d182e1640210c02f98e3df18fda74b8f5ab141abf18":  "active",
				"0:a3935861f79daf59a13d6d182e1640210c02f98e3df18fda74b8f5ab141abf17":  "nonexist",
			},
			wantNames: map[string]string{
				"-1:3333333333333333333333333333333333333333333333333333333333333333": "Elector Contract",
				"-1:5555555555555555555555555555555555555555555555555555555555555555": "Config Contract",
				"0:a3935861f79daf59a13d6d182e1640210c02f98e3df18fda74b8f5ab141abf18":  "Getgems Marketplace",
				"0:a3935861f79daf59a13d6d182e1640210c02f98e3df18fda74b8f5ab141abf17":  "",
			},
		},
		{
			params: oas.OptGetAccountsReq{
				Value: oas.GetAccountsReq{
					AccountIds: []string{
						"-1:3333333333333333333333333333333333333333333333333333333333333333",
						"-1:5555555555555555555555555555555555555555555555555555555555555555",
						"0:a3935861f79daf59a13d6d182e1640210c02f98e3df18fda74b8f5ab141abf18",
						"0:a3935861f79daf59a13d6d182e1640210c02f98e3df18fda74b8f5ab141abf17",
						"0:a3935861f79daf59a13d6d182e1640210c02f98e3df18fda74b8f5ab141abf16",
					},
				},
			},
			wantBadRequestError: "the maximum number of accounts to request at once: 4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
			require.Nil(t, err)
			liteStorage, err := litestorage.NewLiteStorage(logger, cli)
			require.Nil(t, err)
			h := &Handler{
				addressBook: addressbook.NewAddressBook(logger, config.AddressPath, config.JettonPath, config.CollectionPath),
				storage:     liteStorage,
				state:       chainstate.NewChainState(liteStorage),
				limits: Limits{
					BulkLimits: 4,
				},
			}
			accountRes, err := h.GetAccounts(context.Background(), tt.params)
			if len(tt.wantBadRequestError) > 0 {
				badRequest, ok := err.(*oas.ErrorStatusCode)
				require.True(t, ok)
				require.Equal(t, tt.wantBadRequestError, badRequest.Response.Error)
				return
			}
			require.Nil(t, err)

			statuses := map[string]string{}
			names := map[string]string{}
			for _, account := range accountRes.Accounts {
				statuses[account.Address] = string(account.Status)
				names[account.Address] = account.Name.Value
			}
			require.Equal(t, tt.wantStatuses, statuses)
			require.Equal(t, tt.wantNames, names)
		})
	}
}

func TestHandler_GetTransactions(t *testing.T) {
	tests := []struct {
		name           string
		params         oas.GetBlockchainBlockTransactionsParams
		filenamePrefix string
	}{
		{
			name:           "masterchain block",
			params:         oas.GetBlockchainBlockTransactionsParams{BlockID: "(-1,8000000000000000,28741341)"},
			filenamePrefix: "block-txs-1",
		},
		{
			name:           "basechain block",
			params:         oas.GetBlockchainBlockTransactionsParams{BlockID: "(0,8000000000000000,40834551)"},
			filenamePrefix: "block-txs-2",
		},
		{
			name:           "basechain block 2",
			params:         oas.GetBlockchainBlockTransactionsParams{BlockID: "(0,8000000000000000,39478616)"},
			filenamePrefix: "block-txs-3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
			require.Nil(t, err)
			liteStorage, err := litestorage.NewLiteStorage(logger, cli)
			require.Nil(t, err)
			book := &mockAddressBook{
				OnGetAddressInfoByAddress: func(a tongo.AccountID) (addressbook.KnownAddress, bool) {
					return addressbook.KnownAddress{}, false
				},
			}
			h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage), WithAddressBook(book))
			require.Nil(t, err)
			res, err := h.GetBlockchainBlockTransactions(context.Background(), tt.params)
			require.Nil(t, err)

			pkgTesting.CompareResults(t, res, tt.filenamePrefix)
		})
	}
}
