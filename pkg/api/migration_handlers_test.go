package api

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo/ton"

	"github.com/tonkeeper/opentonapi/pkg/core"
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

// TestMigratableJettonBalance pins the single eligibility rule set shared by
// /v2/migration/wallets and /v2/migration/prepare. The "zero balance" case is the regression:
// an already migrated wallet keeps its emptied jetton wallets, and the list endpoint used to
// report them as migratable while prepare correctly ignored them.
func TestMigratableJettonBalance(t *testing.T) {
	whitelisted := oas.JettonBalance{Jetton: oas.JettonPreview{Verification: oas.JettonVerificationTypeWhitelist}}
	blacklisted := oas.JettonBalance{Jetton: oas.JettonPreview{Verification: oas.JettonVerificationTypeBlacklist}}
	convErr := errors.New("no metadata")
	tests := []struct {
		name       string
		wallet     core.JettonWallet
		converted  oas.JettonBalance
		convertErr error
		wantErr    error
		// wantConverts pins the short-circuit: the cheap on-chain checks must run before the
		// expensive conversion, so dust and locked wallets cost no metadata/rate lookups.
		wantConverts int
	}{
		{
			name:         "positive balance, whitelisted",
			wallet:       core.JettonWallet{Balance: decimal.NewFromInt(42)},
			converted:    whitelisted,
			wantConverts: 1,
		},
		{
			name:         "zero balance is not migratable",
			wallet:       core.JettonWallet{Balance: decimal.NewFromInt(0)},
			converted:    whitelisted,
			wantErr:      errJettonNotAvailableForMigration,
			wantConverts: 0,
		},
		{
			name:         "unset balance is not migratable",
			wallet:       core.JettonWallet{},
			converted:    whitelisted,
			wantErr:      errJettonNotAvailableForMigration,
			wantConverts: 0,
		},
		{
			name:         "locked balance",
			wallet:       core.JettonWallet{Balance: decimal.NewFromInt(42), Lock: &core.JettonWalletLockData{FullBalance: decimal.NewFromInt(42), UnlockTime: 1}},
			converted:    whitelisted,
			wantErr:      errJettonNotAvailableForMigration,
			wantConverts: 0,
		},
		{
			name:         "locked and empty",
			wallet:       core.JettonWallet{Lock: &core.JettonWalletLockData{}},
			converted:    whitelisted,
			wantErr:      errJettonNotAvailableForMigration,
			wantConverts: 0,
		},
		{
			name:         "blacklisted",
			wallet:       core.JettonWallet{Balance: decimal.NewFromInt(42)},
			converted:    blacklisted,
			wantErr:      errJettonNotAvailableForMigration,
			wantConverts: 1,
		},
		{
			name:         "conversion failure",
			wallet:       core.JettonWallet{Balance: decimal.NewFromInt(42)},
			convertErr:   convErr,
			wantErr:      convErr,
			wantConverts: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converts := 0
			convert := func(core.JettonWallet) (oas.JettonBalance, error) {
				converts++
				return tt.converted, tt.convertErr
			}
			balance, err := getJettonMigrationBalance(tt.wallet, convert)
			require.Equal(t, tt.wantConverts, converts, "unexpected number of conversions")
			require.Equal(t, tt.wantErr, err)
			if tt.wantErr == nil {
				require.Equal(t, tt.converted, balance)
			} else {
				require.Equal(t, oas.JettonBalance{}, balance)
			}
		})
	}
}

// TestMigrationEligibilityIsShared guards against the two endpoints drifting apart again: the
// wallets the list endpoint keeps must be exactly the wallets prepare builds transfers for.
func TestMigrationEligibilityIsShared(t *testing.T) {
	master := ton.MustParseAccountID("0:97264395bd65a255a429b11326c84128b7d70ffed7949abae3036d506ba38621")
	from := ton.MustParseAccountID("0:945c7be88b0d0c8250cbf42cbffa9137cecc4a98e3581fafa4413cf2dfe2c25d")
	to := ton.MustParseAccountID("0:0000000000000000000000000000000000000000000000000000000000000001")
	wallets := []core.JettonWallet{
		{Address: from, JettonAddress: master, Balance: decimal.NewFromInt(100)},
		{Address: from, JettonAddress: master, Balance: decimal.NewFromInt(0)},
		{Address: from, JettonAddress: master, Balance: decimal.NewFromInt(0)},
		{Address: from, JettonAddress: master, Balance: decimal.NewFromInt(7)},
		{Address: from, JettonAddress: master, Lock: &core.JettonWalletLockData{}, Balance: decimal.NewFromInt(5)},
	}
	convert := func(core.JettonWallet) (oas.JettonBalance, error) {
		return oas.JettonBalance{Jetton: oas.JettonPreview{Verification: oas.JettonVerificationTypeWhitelist}}, nil
	}
	var eligible []migratableJetton
	for _, w := range wallets {
		balance, err := getJettonMigrationBalance(w, convert)
		if err != nil {
			require.ErrorIs(t, err, errJettonNotAvailableForMigration)
			continue
		}
		eligible = append(eligible, migratableJetton{wallet: w, balance: balance})
	}
	require.Len(t, eligible, 2, "only the two non-zero, unlocked wallets are migratable")

	messages, err := prepareJettonTransfers(from, to, eligible, nil, nil)
	require.NoError(t, err)
	require.Len(t, messages, len(eligible), "the list endpoint and the migration plan must agree")
}

func TestHasMigratableAssets(t *testing.T) {
	tests := []struct {
		name   string
		wallet oas.MigrationWalletValue
		want   bool
	}{
		{
			name:   "already migrated wallet has nothing left",
			wallet: oas.MigrationWalletValue{Jettons: []oas.JettonBalance{}},
		},
		{
			name:   "dust below the sweep threshold",
			wallet: oas.MigrationWalletValue{Balance: minGramTransferFee, Jettons: []oas.JettonBalance{}},
		},
		{
			name:   "just above the sweep threshold",
			wallet: oas.MigrationWalletValue{Balance: minGramTransferFee + 1, Jettons: []oas.JettonBalance{}},
			want:   true,
		},
		{
			name:   "jettons only",
			wallet: oas.MigrationWalletValue{Jettons: []oas.JettonBalance{{}}},
			want:   true,
		},
		{
			name:   "nfts only",
			wallet: oas.MigrationWalletValue{Jettons: []oas.JettonBalance{}, NftCount: 1},
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, hasMigratableAssets(tt.wallet))
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
