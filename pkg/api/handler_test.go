package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo"
	"go.uber.org/zap"

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
			liteStorage, err := litestorage.NewLiteStorage([]tongo.AccountID{}, logger)
			require.Nil(t, err)
			h := Handler{
				storage: liteStorage,
			}
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
			liteStorage, err := litestorage.NewLiteStorage([]tongo.AccountID{}, logger)
			require.Nil(t, err)
			h := Handler{
				storage: liteStorage,
			}
			accountRes, err := h.GetAccount(context.Background(), tt.params)
			require.Nil(t, err)
			account, ok := accountRes.(*oas.Account)
			require.True(t, ok)
			require.Equal(t, tt.wantAddress, account.Address)
			require.Equal(t, tt.wantStatus, account.Status)

		})
	}
}
