package api

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/config"
	"github.com/tonkeeper/opentonapi/pkg/litestorage"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func TestHandler_GetRawAccount(t *testing.T) {
	tests := []struct {
		name        string
		params      oas.GetRawAccountParams
		wantStatus  string
		wantAddress string
	}{
		{
			params:      oas.GetRawAccountParams{AccountID: "EQDendoireMDFMufOUzkqNpFIay83GnjV2tgGMbA64wA3siV"},
			wantAddress: "0:de9dda22ade30314cb9f394ce4a8da4521acbcdc69e3576b6018c6c0eb8c00de",
			wantStatus:  "active",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			liteStorage, err := litestorage.NewLiteStorage(logger)
			require.Nil(t, err)
			h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage))
			require.Nil(t, err)
			account, err := h.GetRawAccount(context.Background(), tt.params)
			require.Nil(t, err)
			rawAccount, ok := account.(*oas.RawAccount)
			require.True(t, ok)
			require.Equal(t, tt.wantAddress, rawAccount.Address)
			require.Equal(t, tt.wantStatus, rawAccount.Status)
		})
	}
}

func TestHandler_GetAccount(t *testing.T) {
	tests := []struct {
		name        string
		params      oas.GetAccountParams
		wantStatus  string
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

			liteStorage, err := litestorage.NewLiteStorage(logger)
			require.Nil(t, err)
			h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage))
			require.Nil(t, err)
			accountRes, err := h.GetAccount(context.Background(), tt.params)
			require.Nil(t, err)
			account, ok := accountRes.(*oas.Account)
			require.True(t, ok)
			require.Equal(t, tt.wantAddress, account.Address)
			require.Equal(t, tt.wantStatus, account.Status)
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

			liteStorage, err := litestorage.NewLiteStorage(logger)
			require.Nil(t, err)
			h := Handler{
				addressBook: addressbook.NewAddressBook(logger, config.AddressPath, config.JettonPath, config.CollectionPath),
				storage:     liteStorage,
				limits: Limits{
					BulkLimits: 4,
				},
			}
			accountRes, err := h.GetAccounts(context.Background(), tt.params)
			require.Nil(t, err)
			if len(tt.wantBadRequestError) > 0 {
				badRequest, ok := accountRes.(*oas.BadRequest)
				require.True(t, ok)
				require.Equal(t, tt.wantBadRequestError, badRequest.Error)
				return
			}
			accounts, ok := accountRes.(*oas.Accounts)
			require.True(t, ok)
			_ = accounts
			statuses := map[string]string{}
			names := map[string]string{}
			for _, account := range accounts.Accounts {
				statuses[account.Address] = account.Status
				names[account.Address] = account.Name.Value
			}
			require.Equal(t, tt.wantStatuses, statuses)
			require.Equal(t, tt.wantNames, names)
		})
	}
}

func TestHandler_GetTransactions(t *testing.T) {
	t.Skip() //todo: find better way to test transaction because liteserver can drop old transactions
	tests := []struct {
		name         string
		params       oas.GetBlockTransactionsParams
		wantTxCount  int
		wantTxHashes map[string]struct{}
	}{
		{
			params:      oas.GetBlockTransactionsParams{BlockID: "(-1,8000000000000000,28741341)"},
			wantTxCount: 3,
			wantTxHashes: map[string]struct{}{
				"fec4f77e8b72eec62d14944d9cf99171a32c03783c8e6e30590aabbe35236f9b": {},
				"ed07582702c23aeaa6f1b7ce28def3a810399467a8e062ed1c67eed8c1abd2ad": {},
				"6f268d1fcd0bd36021237bb2b1810a7a78cd1d3b5e96d826ffddbcc96b848523": {},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			liteStorage, err := litestorage.NewLiteStorage(logger)
			require.Nil(t, err)
			h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage))
			require.Nil(t, err)
			res, err := h.GetBlockTransactions(context.Background(), tt.params)
			require.Nil(t, err)
			fmt.Printf("%v\n", res)
			transactions, ok := res.(*oas.Transactions)
			require.True(t, ok)
			require.Equal(t, tt.wantTxCount, len(transactions.Transactions))
			txHashes := map[string]struct{}{}
			for _, tx := range transactions.Transactions {
				txHashes[tx.Hash] = struct{}{}
			}
			require.Equal(t, tt.wantTxHashes, txHashes)
		})
	}
}
