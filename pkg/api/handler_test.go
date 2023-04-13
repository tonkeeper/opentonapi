package api

import (
	"context"
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
	tests := []struct {
		name         string
		params       oas.GetBlockTransactionsParams
		wantTxCount  int
		wantTxHashes map[string]struct{}
	}{
		{
			params:      oas.GetBlockTransactionsParams{BlockID: "(-1,8000000000000000,27906616)"},
			wantTxCount: 5,
			wantTxHashes: map[string]struct{}{
				"9bbc8b4671e944047cae265f962093c65e6d278ac3439242b34ee4d8e063014d": {},
				"739147d0f90ce636ea4ab96797f20193a91a246f4c7203e8ae797f3544311ad1": {},
				"3facfbc5a51c15a183ecdf926f6f96aa90d4d38993b8c0a027a8b076517748f1": {},
				"86676c79ab3b86f5a45bc27e177b1457e40f5d41159e86d0db04ba429d123ac4": {},
				"d5e293051bbe005bd2394bef04596c42f636864c628ff77a71d52baf938d1bbe": {},
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
