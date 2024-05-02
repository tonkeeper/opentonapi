package litestorage

import (
	"context"
	"testing"

	"github.com/puzpuzpuz/xsync/v2"
	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/liteapi"
)

func TestLiteStorage_getAccountInterfaces(t *testing.T) {
	cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
	require.Nil(t, err)
	storage := LiteStorage{
		client:                 cli,
		executor:               cli,
		accountInterfacesCache: xsync.NewTypedMapOf[tongo.AccountID, []abi.ContractInterface](hashAccountID),
	}

	tests := []struct {
		name      string
		accountID tongo.AccountID
		want      []abi.ContractInterface
	}{
		{
			name:      "tep62_item",
			accountID: tongo.MustParseAccountID("0:16b94207124b1613aadd084b65fc2c67bc33e62fbbdd83beef4b194ca2ff5fe7"),
			want:      []abi.ContractInterface{abi.NftItemSimple},
		},
		{
			name:      "get gems",
			accountID: tongo.MustParseAccountID("0:7e809e6484f6af180b18f6760bff96d87c8f27f1fe84e0acb0b48fb86714ed8f"),
			want:      []abi.ContractInterface{abi.NftSaleGetgemsV3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interfaces, err := storage.getAccountInterfaces(context.Background(), tt.accountID)
			require.Nil(t, err)
			require.Equal(t, tt.want, interfaces)
		})
	}
}
