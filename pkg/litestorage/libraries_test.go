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

func TestLiteStorage_GetLibraries(t *testing.T) {
	if os.Getenv("TEST_CI") == "1" {
		t.SkipNow()
		return
	}
	cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
	require.Nil(t, err)

	storage, err := NewLiteStorage(zap.L(), cli)
	require.Nil(t, err)

	libs := []tongo.Bits256{
		tongo.MustParseHash("587CC789EFF1C84F46EC3797E45FC809A14FF5AE24F1E0C7A6A99CC9DC9061FF"),
	}
	libraries, err := storage.GetLibraries(context.Background(), libs)
	require.Nil(t, err)
	require.Equal(t, 1, len(libraries))

	keys := storage.tvmLibraryCache.Keys()
	expected := []string{"587cc789eff1c84f46ec3797e45fc809a14ff5ae24f1e0c7a6a99cc9dc9061ff"}
	require.Equal(t, expected, keys)

	// we need to shut down storage to disable fetching blockchain config
	storage.Shutdown()

	// second call should take libraries from cache
	storage.client = nil

	libraries, err = storage.GetLibraries(context.Background(), libs)
	require.Nil(t, err)
	require.Equal(t, 1, len(libraries))

}
