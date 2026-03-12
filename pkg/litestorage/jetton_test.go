package litestorage

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteapi"
	"go.uber.org/zap"
)

// This test relies on public mainnet data and is skipped in CI.
func TestGetJettonMasterData_PopulatesHashes(t *testing.T) {
	if os.Getenv("TEST_CI") == "1" {
		t.SkipNow()
		return
	}

	logger, _ := zap.NewDevelopment()
	cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
	require.NoError(t, err)

	storage, err := NewLiteStorage(logger, cli)
	require.NoError(t, err)

	// Use a well-known jetton master (KINGYTON from existing tests).
	master := tongo.MustParseAddress("0:beb5d4638e860ccf7317296e298fde5b35982f4725b0676dc98b1de987b82ebc").ID

	data, err := storage.GetJettonMasterData(context.Background(), master)
	require.NoError(t, err)

	require.NotEmpty(t, data.CodeHash, "CodeHash should be populated for jetton master")
	require.NotEmpty(t, data.DataHash, "DataHash should be populated for jetton master")
	require.NotZero(t, data.LastTransactionLt, "LastTransactionLt should be populated for jetton master")
}
