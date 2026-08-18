package api

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	tonwallet "github.com/tonkeeper/tongo/wallet"

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

	messages, err := prepareJettonTransfers(prepareJettonTransfersParams{
		from: from, to: to, excessDestination: to, jettons: eligible,
	})
	require.NoError(t, err)
	require.Len(t, messages, len(eligible), "the list endpoint and the migration plan must agree")
}

// decodedResponseDestination unmarshals a wallet.RawMessage built for a jetton transfer and
// returns the response_destination/excess address encoded in its body.
func decodedResponseDestination(t *testing.T, msg tonwallet.RawMessage) ton.AccountID {
	var m abi.MessageRelaxed
	msg.Message.ResetCounters()
	require.NoError(t, tlb.Unmarshal(msg.Message, &m))
	body, ok := m.MessageInternal.Body.Value.Value.(abi.JettonTransferMsgBody)
	require.True(t, ok, "expected a jetton transfer body, got %T", m.MessageInternal.Body.Value.Value)
	id, err := ton.AccountIDFromTlb(body.ResponseDestination)
	require.NoError(t, err)
	require.NotNil(t, id, "response_destination must be a real address")
	return *id
}

// TestJettonTransferRawMessage_ExcessDestination pins the fix for battery migrations bleeding
// relay funds: a battery-sponsored jetton transfer must return its unspent gas to the relay,
// which fronted it, not to the new wallet — otherwise the relay loses the full attached amount on
// every sponsored transfer instead of just what it actually cost.
func TestJettonTransferRawMessage_ExcessDestination(t *testing.T) {
	from := ton.MustParseAccountID("0:945c7be88b0d0c8250cbf42cbffa9137cecc4a98e3581fafa4413cf2dfe2c25d")
	to := ton.MustParseAccountID("0:0000000000000000000000000000000000000000000000000000000000000001")
	senderJettonWallet := ton.MustParseAccountID("0:0000000000000000000000000000000000000000000000000000000000000002")
	relay := ton.MustParseAccountID("0:0000000000000000000000000000000000000000000000000000000000000004")

	t.Run("self-paid sends excess to the new wallet", func(t *testing.T) {
		msg, err := jettonTransferRawMessage(from, to, to, senderJettonWallet, big.NewInt(100))
		require.NoError(t, err)
		require.Equal(t, to, decodedResponseDestination(t, msg))
	})
	t.Run("battery-sponsored sends excess back to the relay", func(t *testing.T) {
		msg, err := jettonTransferRawMessage(from, to, relay, senderJettonWallet, big.NewInt(100))
		require.NoError(t, err)
		require.Equal(t, relay, decodedResponseDestination(t, msg))
	})
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

func TestRequiredGas(t *testing.T) {
	transfer := tonwallet.RawMessage{Mode: tonwallet.DefaultMessageMode}
	sweep := tonwallet.RawMessage{Mode: migrationSweepMode}
	tests := []struct {
		name string
		plan migrationPlan
		want tlb.Grams
	}{
		{
			name: "empty plan needs nothing",
		},
		{
			name: "sweep-only plan attaches no gas",
			plan: []migrationBatch{{messages: []tonwallet.RawMessage{sweep}}},
		},
		{
			name: "self-paid transfers are charged per message",
			plan: []migrationBatch{
				{messages: []tonwallet.RawMessage{transfer, transfer}},
				{messages: []tonwallet.RawMessage{transfer}},
				{messages: []tonwallet.RawMessage{sweep}},
			},
			want: 3 * migrationGasPerTransfer,
		},
		{
			name: "sponsored batches are funded by the relay",
			plan: []migrationBatch{
				{messages: []tonwallet.RawMessage{transfer, transfer}, sponsored: true},
				{messages: []tonwallet.RawMessage{sweep}},
			},
		},
		{
			name: "mixed plan charges only the unsponsored part",
			plan: []migrationBatch{
				{messages: []tonwallet.RawMessage{transfer, transfer}, sponsored: true},
				{messages: []tonwallet.RawMessage{transfer}},
				{messages: []tonwallet.RawMessage{sweep}},
			},
			want: migrationGasPerTransfer,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.plan.minGramBalanceRequired())
		})
	}
}

// TestAssembleMigrationPlan_SweepSponsorship pins the fix for a battery-eligible v5 wallet that
// holds only TON: the plan then consists solely of the final sweep batch, so unless that batch
// itself can be marked sponsored, the client sees sponsored:false everywhere and hides battery
// even though it applies. It also pins the battery-sponsored sweep split (payout to the new
// wallet, then a mode-128 reclaim to the relay) that keeps the relay's fronted gas from leaking
// to the destination.
func TestAssembleMigrationPlan_SweepSponsorship(t *testing.T) {
	to := ton.MustParseAccountID("0:0000000000000000000000000000000000000000000000000000000000000001")
	relay := ton.MustParseAccountID("0:0000000000000000000000000000000000000000000000000000000000000002")
	tests := []struct {
		name           string
		relayFunded    bool
		sweepSponsored bool
		wantSponsored  bool
		wantModes      []byte
	}{
		{name: "self-paid: sweep stays unsponsored, single mode-128 transfer", wantModes: []byte{migrationSweepMode}},
		{name: "gasless: sweep has no jetton to bill, stays unsponsored", relayFunded: true, wantModes: []byte{migrationSweepMode}},
		{
			name:        "battery: sweep is sponsored, payout to the new wallet then a mode-128 reclaim to the relay",
			relayFunded: true, sweepSponsored: true, wantSponsored: true,
			wantModes: []byte{tonwallet.DefaultMessageMode, migrationSweepMode},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := assembleMigrationPlan(assembleMigrationPlanParams{
				to: to, excessDestination: relay, relayFunded: tt.relayFunded, sweepSponsored: tt.sweepSponsored,
				gramBalance: minGramTransferFee + 1,
			})
			require.NoError(t, err)
			require.Len(t, plan, 1, "a TON-only wallet's plan is just the sweep batch")
			require.Equal(t, tt.wantSponsored, plan[0].sponsored)
			require.Len(t, plan[0].messages, len(tt.wantModes))
			for i, mode := range tt.wantModes {
				require.Equal(t, mode, plan[0].messages[i].Mode, "message %d mode", i)
			}
		})
	}
}

func TestGasFundedMessages(t *testing.T) {
	batch := migrationBatch{messages: []tonwallet.RawMessage{
		{Mode: tonwallet.DefaultMessageMode},
		{Mode: migrationSweepMode},
		{Mode: tonwallet.DefaultMessageMode},
	}}
	require.Equal(t, 2, batch.gasFundedMessages(), "the balance-carrying sweep needs no attached gas")
}

// TestMigrationValidUntilFollowsEmulationClock pins the fix for migrations that died partway down the
// plan on v3/v4 sources. Those wallets carry only four messages per transaction, so a wallet with a
// few dozen assets becomes a dozen-plus batches, and the emulator moves its clock forward by the chain
// time every batch takes. A deadline pinned to the moment prepare was called expired mid-plan and the
// wallet rejected the remaining batches with exit code 36. Each batch's deadline must instead sit a
// full lifetime ahead of the clock that batch starts on.
func TestMigrationValidUntilFollowsEmulationClock(t *testing.T) {
	start := time.Now().Unix()
	// A v3/v4 plan for >50 assets: 4 messages per transaction plus the final sweep. Per-batch drift is
	// the emulator's inter-shard delay times the rounds a jetton transfer takes to settle.
	const batches = 50/4 + 1 + 1
	const driftPerBatch = 8 * 4

	emuTime := start
	for i := range batches {
		validUntil := migrationValidUntil(emuTime)
		require.Truef(t, validUntil.Unix() > emuTime,
			"batch %v is already expired on the clock it starts on: valid_until %v, now %v", i, validUntil.Unix(), emuTime)
		emuTime += driftPerBatch
	}

	// The old behaviour — one deadline for the whole plan — is what this guards against.
	fixed := time.Unix(start, 0).Add(migrationMsgLifetime)
	require.Less(t, fixed.Unix(), emuTime,
		"the test is not exercising the regression: the plan no longer outruns a fixed deadline")
}

var testMigrationSource = ton.MustParseAccountID("0:0000000000000000000000000000000000000000000000000000000000000001")

func testEmulatedStates(balance tlb.Grams) map[ton.AccountID]tlb.ShardAccount {
	var account tlb.ShardAccount
	account.Account.SumType = "Account"
	account.Account.Account.Storage.Balance.Grams = balance
	return map[ton.AccountID]tlb.ShardAccount{testMigrationSource: account}
}

// TestWalletSpend pins the accounting behind the emulated gas price: what the wallet parted with is
// the difference between the balance it started on and the one the emulator left it at — the
// emulated state already nets off the excess the transfers send back.
func TestWalletSpend(t *testing.T) {
	spent, err := gramBalanceSpent(testEmulatedStates(7*ton.OneGRAM), testMigrationSource, 8*ton.OneGRAM)
	require.NoError(t, err)
	require.EqualValues(t, ton.OneGRAM, spent)

	// A relay-funded batch credits the wallet, which must not wrap around into a huge spend.
	spent, err = gramBalanceSpent(testEmulatedStates(9*ton.OneGRAM), testMigrationSource, 8*ton.OneGRAM)
	require.NoError(t, err)
	require.EqualValues(t, -int64(ton.OneGRAM), spent)

	_, err = gramBalanceSpent(map[ton.AccountID]tlb.ShardAccount{}, testMigrationSource, ton.OneGRAM)
	require.Error(t, err, "a missing source state must not pass as a zero spend")

	var uninit tlb.ShardAccount
	uninit.Account.SumType = "AccountNone"
	_, err = gramBalanceSpent(map[ton.AccountID]tlb.ShardAccount{testMigrationSource: uninit}, testMigrationSource, ton.OneGRAM)
	require.Error(t, err, "an uninitialized source must not pass as a full spend")
}

// TestDropOverfunding pins the step that keeps the over-funding out of the response: the sweep, and
// so the amount the user is shown, must be emulated against the balance the wallet really has.
func TestDropOverfunding(t *testing.T) {
	source := testMigrationSource

	// The wallet started 6 TON over-funded and spent 1 TON of its own, so 2 TON is what is really left.
	emulated := testEmulatedStates(8 * ton.OneGRAM)
	require.NoError(t, deductGramBalance(emulated, source, 6*ton.OneGRAM))
	require.Equal(t, 2*ton.OneGRAM, emulated[source].Account.Account.Storage.Balance.Grams)

	require.Error(t, deductGramBalance(map[ton.AccountID]tlb.ShardAccount{}, source, ton.OneGRAM),
		"a missing source state must not pass silently")

	require.Error(t, deductGramBalance(testEmulatedStates(ton.OneGRAM), source, 6*ton.OneGRAM),
		"a balance below the over-funding means the wallet spent into it")
}

// TestDropOverfundFromBalances pins that the over-funding never reaches the reported trace: the
// balance of every source transaction in it, at any depth, is the one the wallet really ends on.
func TestDropOverfundFromBalances(t *testing.T) {
	other := ton.MustParseAccountID("0:0000000000000000000000000000000000000000000000000000000000000002")
	tx := func(account ton.AccountID, endBalance int64, children ...*core.Trace) *core.Trace {
		return &core.Trace{
			Transaction: core.Transaction{
				TransactionID: core.TransactionID{Account: account},
				EndBalance:    endBalance,
			},
			Children: children,
		}
	}
	// The wallet tx, a jetton wallet in between, and the excess coming back to the source.
	trace := tx(testMigrationSource, 7*int64(ton.OneGRAM),
		tx(other, 3*int64(ton.OneGRAM),
			tx(testMigrationSource, 7*int64(ton.OneGRAM))))

	dropOverfundFromBalances(trace, testMigrationSource, 6*ton.OneGRAM)

	require.EqualValues(t, ton.OneGRAM, trace.EndBalance)
	require.EqualValues(t, 3*ton.OneGRAM, trace.Children[0].EndBalance, "other accounts are untouched")
	require.EqualValues(t, ton.OneGRAM, trace.Children[0].Children[0].EndBalance, "a refund to the source counts too")

	// An over-funding above the reported balance must not wrap the balance around.
	shallow := tx(testMigrationSource, int64(ton.OneGRAM))
	dropOverfundFromBalances(shallow, testMigrationSource, 6*ton.OneGRAM)
	require.Zero(t, shallow.EndBalance)

	dropOverfundFromBalances(nil, testMigrationSource, ton.OneGRAM) // must not panic
}

// TestEmulationOutcomeError guards the hole that let a failed batch into a 200 response: the
// emulator reports an aborted transaction as a successful emulation, so the outcome must be checked.
func TestEmulationOutcomeError(t *testing.T) {
	succeeded := &core.Trace{Transaction: core.Transaction{Success: true}}
	require.NoError(t, traceError(succeeded, 3))

	aborted := &core.Trace{Transaction: core.Transaction{
		Aborted:      true,
		ComputePhase: &core.TxComputePhase{Success: true, ExitCode: 0},
		ActionPhase:  &core.TxActionPhase{ResultCode: 37},
	}}
	err := traceError(aborted, 3)
	require.ErrorIs(t, err, errEmulationFailed)
	require.Contains(t, err.Error(), "seqno 3")
	require.Contains(t, err.Error(), "action result code 37")
	require.NotContains(t, err.Error(), "compute exit code", "a compute phase that succeeded is not a reason")

	// Running out of gas skips the compute phase, which leaves exit code 0 behind: report the reason.
	outOfGas := &core.Trace{Transaction: core.Transaction{
		Aborted: true,
		ComputePhase: &core.TxComputePhase{
			Skipped:    true,
			SkipReason: tlb.ComputeSkipReasonNoGas,
		},
	}}
	err = traceError(outOfGas, 0)
	require.ErrorIs(t, err, errEmulationFailed)
	require.Contains(t, err.Error(), string(tlb.ComputeSkipReasonNoGas))
	require.NotContains(t, err.Error(), "exit code 0")

	// No phase owns up to the failure: it must still be reported, not answered with a 200.
	phaseless := &core.Trace{Transaction: core.Transaction{Aborted: true}}
	err = traceError(phaseless, 1)
	require.ErrorIs(t, err, errEmulationFailed)
	require.Contains(t, err.Error(), "aborted=true")

	// Skipped actions do not abort the batch (+2 SendIgnoreErrors) and are only reported in the
	// emulated trace (skipped_actions), not rejected.
	skipped := &core.Trace{Transaction: core.Transaction{
		Success:     true,
		ActionPhase: &core.TxActionPhase{Success: true, TotalActions: 4, SkippedActions: 1},
	}}
	require.NoError(t, traceError(skipped, 4))

	// A trace-less emulation is a failure, not a pass, and must not panic on the way out.
	err = traceError(nil, 2)
	require.ErrorIs(t, err, errEmulationFailed)
	require.Contains(t, err.Error(), "seqno 2")
}

// oneGram is ton.OneGRAM as the int64 nanoton amount the shortfall helpers work in.
const oneGram = int64(ton.OneGRAM)

// TestMigrationConflict pins the contract of the insufficient-gas response: a wallet that only lacks
// gas gets the whole prepared migration back, on the same body as the shortfall, so it does not have
// to re-request prepare after topping up. The JSON assertions guard the shape itself — the fields must
// stay at the top level, i.e. exactly where the success response has them.
func TestMigrationConflict(t *testing.T) {
	resp := &oas.MigrationPrepareResponse{
		From:          "0:97264395bd65a255a429b11326c84128b7d70ffed7949abae3036d506ba38621",
		To:            "0:97264395bd65a255a429b11326c84128b7d70ffed7949abae3036d506ba38622",
		WalletVersion: "v5R1",
		Transactions: []oas.MigrationTransaction{
			{Seqno: 0, Boc: "te6first", GasSpent: 4321000},
			{Seqno: 1, Boc: "te6second", GasSpent: 1234000},
		},
	}
	conflict := migrationConflict(resp, InsufficientFunds{Required: 3 * oneGram, Available: oneGram})

	require.Equal(t, resp.From, conflict.From)
	require.Equal(t, resp.To, conflict.To)
	require.Equal(t, resp.WalletVersion, conflict.WalletVersion)
	require.Equal(t, resp.Transactions, conflict.Transactions, "the whole plan travels with the shortfall")
	require.Equal(t, "insufficient GRAM for gas", conflict.Error)
	require.True(t, conflict.ErrorCode.Set)
	require.EqualValues(t, 50000, conflict.ErrorCode.Value, "the extended code clients switch on")
	require.True(t, conflict.Details.Set)
	require.EqualValues(t, 3*oneGram, conflict.Details.Value.Required)
	require.EqualValues(t, oneGram, conflict.Details.Value.Available)

	raw, err := json.Marshal(conflict)
	require.NoError(t, err)
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &body))
	for _, field := range []string{"from", "to", "wallet_version", "transactions", "error", "error_code", "details"} {
		require.Contains(t, body, field, "%v must be a top-level field", field)
	}
	require.JSONEq(t, `{"required":3000000000,"available":1000000000}`, string(body["details"]))
	var encodedTransactions []json.RawMessage
	require.NoError(t, json.Unmarshal(body["transactions"], &encodedTransactions))
	require.Len(t, encodedTransactions, 2, "every transaction is serialized, not just the first")
	var firstTransaction map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encodedTransactions[0], &firstTransaction))
	require.JSONEq(t, "4321000", string(firstTransaction["gas_spent"]), "the fee is reported per transaction")
}

// TestInsufficientGramForGasError covers the bare 409, still returned when the plan cannot be built:
// an uninitialized source cannot be over-funded for emulation, and a failure after a shortfall was
// found must report the shortfall rather than its consequence.
func TestInsufficientGramForGasError(t *testing.T) {
	err := errInsufficientGramForGas(3*oneGram, oneGram)
	statusCode, ok := err.(*oas.ErrorStatusCode)
	require.True(t, ok, "expected *oas.ErrorStatusCode, got %T", err)
	require.Equal(t, http.StatusConflict, statusCode.StatusCode)
	require.Equal(t, "insufficient GRAM for gas", statusCode.Response.Error)
	require.True(t, statusCode.Response.ErrorCode.Set)
	require.EqualValues(t, 50000, statusCode.Response.ErrorCode.Value)
	require.True(t, statusCode.Response.Details.Set)
	require.EqualValues(t, 3*oneGram, statusCode.Response.Details.Value.Required)
	require.EqualValues(t, oneGram, statusCode.Response.Details.Value.Available)
}

// TestWorseShortfall pins the number the client is told to top up to. The static plan minimum only
// counts the gas attached to the transfers; the emulated spend also covers what the wallet itself
// burns, so the larger of the two must win or the client tops up and fails again.
func TestWorseShortfall(t *testing.T) {
	require.Nil(t, worseShortfall(nil, oneGram, oneGram), "a covered requirement is no shortfall")
	require.Nil(t, worseShortfall(nil, oneGram, 2*oneGram))

	found := worseShortfall(nil, 2*oneGram, oneGram)
	require.NotNil(t, found)
	require.EqualValues(t, 2*oneGram, found.Required)
	require.EqualValues(t, oneGram, found.Available)

	// The emulated cost exceeds the static minimum: report the bigger one.
	worse := worseShortfall(found, 3*oneGram, oneGram)
	require.EqualValues(t, 3*oneGram, worse.Required)

	// A smaller or equal later observation must not shrink the requirement.
	require.Same(t, worse, worseShortfall(worse, 2*oneGram, oneGram))
	require.Same(t, worse, worseShortfall(worse, 3*oneGram, oneGram))
	// A batch the source can cover does not clear a shortfall already found.
	require.Same(t, worse, worseShortfall(worse, 0, oneGram))
}

// TestTraceFees pins the fee reported as gas_spent: every transaction in the trace contributes, and an
// action phase replaces its total_action_fees — already inside total_fees — with the full forward fee,
// so the part collected downstream when the message is imported is counted too.
func TestTraceFees(t *testing.T) {
	trace := func(totalFee int64, phase *core.TxActionPhase, children ...*core.Trace) *core.Trace {
		return &core.Trace{
			Transaction: core.Transaction{TotalFee: totalFee, ActionPhase: phase},
			Children:    children,
		}
	}
	require.Zero(t, traceFees(nil), "a missing trace costs nothing")
	require.EqualValues(t, 1000, traceFees(trace(1000, nil)), "no action phase, no children")

	// 1000 + (400 - 100) on the wallet, 700 on the jetton wallet, 300 on the destination.
	full := trace(1000, &core.TxActionPhase{FwdFees: 400, TotalFees: 100},
		trace(700, nil, trace(300, nil)),
	)
	require.EqualValues(t, 2300, traceFees(full))
}

func requireBadRequestPrefix(t *testing.T, err error, prefix string) {
	t.Helper()
	require.Error(t, err)
	badRequest, ok := err.(*oas.ErrorStatusCode)
	require.True(t, ok, "expected *oas.ErrorStatusCode, got %T", err)
	require.Equal(t, 400, badRequest.StatusCode)
	require.Contains(t, badRequest.Response.Error, prefix)
}
