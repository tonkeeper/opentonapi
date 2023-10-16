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
		approvers []oas.NftApprovedByItem
		want      []oas.NftApprovedByItem
	}{
		{
			name:      "all good",
			approvers: []oas.NftApprovedByItem{oas.NftApprovedByItemTonkeeper, oas.NftApprovedByItemGetgems, oas.NftApprovedByItemGetgems, oas.NftApprovedByItemTonkeeper},
			want:      []oas.NftApprovedByItem{oas.NftApprovedByItemGetgems, oas.NftApprovedByItemTonkeeper},
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
