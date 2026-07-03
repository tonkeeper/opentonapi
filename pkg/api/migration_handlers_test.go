package api

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	tongoWallet "github.com/tonkeeper/tongo/wallet"
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

func TestMaxMessagesForVersion(t *testing.T) {
	require.Equal(t, 4, walletMaxMessageCount(tongoWallet.V3R2))
	require.Equal(t, 4, walletMaxMessageCount(tongoWallet.V4R2))
	require.Equal(t, 255, walletMaxMessageCount(tongoWallet.V5R1))
}

func TestChunkMessages(t *testing.T) {
	mk := func(n int) []tongoWallet.RawMessage {
		out := make([]tongoWallet.RawMessage, n)
		return out
	}
	require.Empty(t, chunkMessages(nil, 4))

	// 9 messages, batches of 4 => 4 + 4 + 1
	batches := chunkMessages(mk(9), 4)
	require.Len(t, batches, 3)
	require.Len(t, batches[0], 4)
	require.Len(t, batches[1], 4)
	require.Len(t, batches[2], 1)

	// a single batch when everything fits (v5)
	require.Len(t, chunkMessages(mk(200), 255), 1)
}

func TestChunkMessages_SweepIsLast(t *testing.T) {
	to := ton.MustParseAccountID("0:97264395bd65a255a429b11326c84128b7d70ffed7949abae3036d506ba38621")
	jetton := ton.MustParseAccountID("0:0000000000000000000000000000000000000000000000000000000000000001")

	var messages []tongoWallet.RawMessage
	for range 5 {
		jm, err := walletJettonTransferMessage(jetton, to, big.NewInt(1000))
		require.NoError(t, err)
		raw, err := toWalletRawMessage(jm)
		require.NoError(t, err)
		messages = append(messages, raw)
	}
	sweep, err := toWalletRawMessage(tongoWallet.Message{Amount: 0, Address: to, Mode: migrationSweepMode})
	require.NoError(t, err)
	messages = append(messages, sweep)

	batches := chunkMessages(messages, 4) // v4: 6 msgs => [4][2]
	require.Len(t, batches, 2)
	last := batches[len(batches)-1]
	require.Equal(t, byte(migrationSweepMode), last[len(last)-1].Mode, "TON sweep must be the last message")
}

func TestJettonTransferMessage(t *testing.T) {
	jettonWallet := ton.MustParseAccountID("0:0000000000000000000000000000000000000000000000000000000000000001")
	destination := ton.MustParseAccountID("0:97264395bd65a255a429b11326c84128b7d70ffed7949abae3036d506ba38621")
	amount := big.NewInt(123456789)

	msg, err := walletJettonTransferMessage(jettonWallet, destination, amount)
	require.NoError(t, err)
	require.Equal(t, jettonWallet, msg.Address)
	require.Equal(t, migrationGasPerTransfer, msg.Amount)
	require.Equal(t, byte(tongoWallet.DefaultMessageMode), msg.Mode)

	body := msg.Body
	body.ResetCounters()
	op, err := body.ReadUint(32)
	require.NoError(t, err)
	require.Equal(t, uint64(0xf8a7ea5), op)
	var decoded abi.JettonTransferMsgBody
	require.NoError(t, tlb.Unmarshal(body, &decoded))
	require.Equal(t, amount.String(), (*big.Int)(&decoded.Amount).String())
	gotDest, err := ton.AccountIDFromTlb(decoded.Destination)
	require.NoError(t, err)
	require.NotNil(t, gotDest)
	require.Equal(t, destination, *gotDest)
}

func TestNftTransferMessage(t *testing.T) {
	item := ton.MustParseAccountID("0:0000000000000000000000000000000000000000000000000000000000000002")
	destination := ton.MustParseAccountID("0:97264395bd65a255a429b11326c84128b7d70ffed7949abae3036d506ba38621")

	msg := walletNFTTransferMessage(item, destination)
	require.Equal(t, item, msg.Address)

	body := msg.Body
	body.ResetCounters()
	op, err := body.ReadUint(32)
	require.NoError(t, err)
	require.Equal(t, uint64(0x5fcc3d14), op)
	var decoded abi.NftTransferMsgBody
	require.NoError(t, tlb.Unmarshal(body, &decoded))
	gotOwner, err := ton.AccountIDFromTlb(decoded.NewOwner)
	require.NoError(t, err)
	require.NotNil(t, gotOwner)
	require.Equal(t, destination, *gotOwner)
}

func TestBuildUnsignedBodyV4(t *testing.T) {
	to := ton.MustParseAccountID("0:97264395bd65a255a429b11326c84128b7d70ffed7949abae3036d506ba38621")
	sweep, err := toWalletRawMessage(tongoWallet.Message{Amount: 0, Address: to, Mode: migrationSweepMode})
	require.NoError(t, err)
	messages := []tongoWallet.RawMessage{sweep}

	const seqno = 12
	const subWalletID = 698983191
	body, err := buildUnsignedBody(tongoWallet.V4R2, subWalletID, nil, 0, seqno, time.Unix(1900000000, 0), messages)
	require.NoError(t, err)

	body.ResetCounters()
	var decoded tongoWallet.MessageV4
	require.NoError(t, tlb.Unmarshal(body, &decoded))
	require.Equal(t, uint32(seqno), decoded.Seqno)
	require.Equal(t, uint32(subWalletID), decoded.SubWalletId)
	require.Equal(t, int8(0), decoded.Op)
	require.Len(t, decoded.RawMessages, len(messages))
}

func TestSignedBodyForEmulation(t *testing.T) {
	to := ton.MustParseAccountID("0:97264395bd65a255a429b11326c84128b7d70ffed7949abae3036d506ba38621")
	sweep, err := toWalletRawMessage(tongoWallet.Message{Amount: 0, Address: to, Mode: migrationSweepMode})
	require.NoError(t, err)
	body, err := buildUnsignedBody(tongoWallet.V4R2, 698983191, nil, 0, 1, time.Unix(1900000000, 0), []tongoWallet.RawMessage{sweep})
	require.NoError(t, err)

	signed, err := signedBodyForEmulation(tongoWallet.V4R2, body)
	require.NoError(t, err)
	// v3/v4 prepend a 512-bit signature in front of the body bits.
	require.Equal(t, body.BitsAvailableForRead()+512, signed.BitsAvailableForRead())
}

func TestSignedBodyForEmulationV5(t *testing.T) {
	to := ton.MustParseAccountID("0:97264395bd65a255a429b11326c84128b7d70ffed7949abae3036d506ba38621")
	sweep, err := toWalletRawMessage(tongoWallet.Message{Amount: 0, Address: to, Mode: migrationSweepMode})
	require.NoError(t, err)
	body, err := buildUnsignedBody(tongoWallet.V5R1, 0, make([]byte, 32), 0, 1, time.Unix(1900000000, 0), []tongoWallet.RawMessage{sweep})
	require.NoError(t, err)

	signed, err := signedBodyForEmulation(tongoWallet.V5R1, body)
	require.NoError(t, err)
	// v5 carries its signature as the trailing 512 bits; the unsigned body must stay untouched
	// while emulation gets a copy with a zero placeholder appended.
	require.Equal(t, body.BitsAvailableForRead()+512, signed.BitsAvailableForRead())
	require.Equal(t, body.RefsSize(), signed.RefsSize())
}
