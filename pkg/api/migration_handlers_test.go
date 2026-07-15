package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func TestHandler_GetMigrationWallets_Validation(t *testing.T) {
	h := &Handler{limits: Limits{BulkLimits: 4}}
	tests := []struct {
		name          string
		ids           []string
		wantErrPrefix string
	}{
		{
			name:          "empty list",
			ids:           []string{},
			wantErrPrefix: "empty list of ids",
		},
		{
			name:          "over the bulk limit",
			ids:           []string{"0:00", "0:01", "0:02", "0:03", "0:04"},
			wantErrPrefix: "the maximum number of accounts to request at once: 4",
		},
		{
			name:          "invalid address",
			ids:           []string{"not-an-address"},
			wantErrPrefix: "can't decode address",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := oas.OptGetMigrationWalletsReq{
				Set:   true,
				Value: oas.GetMigrationWalletsReq{AccountIds: tt.ids},
			}
			_, err := h.GetMigrationWallets(context.Background(), req, oas.GetMigrationWalletsParams{})
			requireBadRequestPrefix(t, err, tt.wantErrPrefix)
		})
	}
}

func TestHandler_PrepareMigration_Validation(t *testing.T) {
	h := &Handler{}
	tests := []struct {
		name          string
		from          string
		to            string
		wantErrPrefix string
	}{
		{
			name:          "invalid from",
			from:          "not-an-address",
			to:            "0:97264395bd65a255a429b11326c84128b7d70ffed7949abae3036d506ba38621",
			wantErrPrefix: "invalid `from` address",
		},
		{
			name:          "invalid to",
			from:          "0:97264395bd65a255a429b11326c84128b7d70ffed7949abae3036d506ba38621",
			to:            "not-an-address",
			wantErrPrefix: "invalid `to` address",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := h.PrepareMigration(context.Background(), &oas.MigrationPrepareRequest{From: tt.from, To: tt.to})
			requireBadRequestPrefix(t, err, tt.wantErrPrefix)
		})
	}
}

func requireBadRequestPrefix(t *testing.T, err error, prefix string) {
	t.Helper()
	require.Error(t, err)
	badRequest, ok := err.(*oas.ErrorStatusCode)
	require.True(t, ok, "expected *oas.ErrorStatusCode, got %T", err)
	require.Equal(t, 400, badRequest.StatusCode)
	require.Contains(t, badRequest.Response.Error, prefix)
}
