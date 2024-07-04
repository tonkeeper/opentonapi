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
					Root:   "b5ee9c7201020b01000274000114ff00f4a413f4bcf2c80b010202d102030201200405020120080901cb0cf434c0c05c6c08f1c02c6497c0f83e900c3e1b7c007e12fcb8b0c074c7c860841315adad6ea38ecca0840de7bd4eeea38bbe137e12b1c17cb8afbe900c1c1cc860063232c17e1173c5887e80b2dab2c7f2cfc073c5b260103ec01c3e1afc00a44c38b8c3600600753b513434ffc07e1874c7c07e18be80007e18f4c7c07e193e90007e1974c7c07e19bd01007e19f4c7c07e1a34c7c07e1a7e90007e1ab4800c3e1ae001fc31f823f848bef823f849bbb0f2e2c0d30f8308d71820f901f8414130f910f2e2bdd33f01f86cfa4030f84d21c705f2e2befa4401c000f2e2c2f8425220bc9431f84201def84752108307f40e6fa170019430d31f309131e2f84621a15230bc9632f84622a102de22c200f2e2c1821007270e00238100f9a08100faa904a80700eef843820afaf080a05240a8a05340bef2e2bff84224a1f8625113a0c8cb1fc9d0f84741308307f416f86721f0035320a18209312d00bc8e16708018c8cb05f84dcf165042a1fa0212cb6ac970fb00923031e2f8435210a8c2008e17708018c8cb05f84acf16f84313a812fa02cb6ac971fb009130e2f00200613e12fe127e123e11fe11be113e10be107232fff2c7fe10fe80b2c7fe1173c5b2c7fd0032c7f2c7fe12b3c5b280327b5520010f24c8308022ba0c200a00f06d218100fab608208e43c8f84470019d7aa90ca630021023a40120c000e63092cb07e48b52e6a736f6e8cf16c9c8f84dcf16ccc9c8820afaf080fa02ccc9d0f84455028040f416f844a4f86401e4f84c72708018c8cb05f845cf16820b93870005a415a814fa0213cb6a12cb1fcb3fccc971fb008100faa1",
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
		req                 oas.OptGetAccountsReq
		wantStatuses        map[string]string
		wantNames           map[string]string
		wantBadRequestError string
	}{
		{
			req: oas.OptGetAccountsReq{
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
			req: oas.OptGetAccountsReq{
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
			accountRes, err := h.GetAccounts(context.Background(), tt.req, oas.GetAccountsParams{})
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
