package addressbook

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo/ton"
)

func Test_unique(t *testing.T) {
	tests := []struct {
		name      string
		approvers []oas.NftItemApprovedByItem
		want      []oas.NftItemApprovedByItem
	}{
		{
			name:      "all good",
			approvers: []oas.NftItemApprovedByItem{oas.NftItemApprovedByItemTonkeeper, oas.NftItemApprovedByItemGetgems, oas.NftItemApprovedByItemGetgems, oas.NftItemApprovedByItemTonkeeper},
			want:      []oas.NftItemApprovedByItem{oas.NftItemApprovedByItemGetgems, oas.NftItemApprovedByItemTonkeeper},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newList := unique(tt.approvers)
			if !reflect.DeepEqual(newList, tt.want) {
				t.Errorf("unique() = %v, want %v", newList, tt.want)
			}
		})
	}
}

func TestFetchGetGemsVerifiedCollections(t *testing.T) {
	accountIDs, err := FetchGetGemsVerifiedCollections()
	require.Nil(t, err)
	m := make(map[ton.AccountID]struct{})
	for _, accountID := range accountIDs {
		m[accountID] = struct{}{}
	}
	require.Equal(t, len(m), len(accountIDs))
	require.True(t, len(m) > 100)
}
