package addressbook

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo/ton"
)

func TestFetchGetGemsVerifiedCollections(t *testing.T) {
	accountIDs, err := fetchGetGemsVerifiedCollections()
	require.Nil(t, err)
	m := make(map[ton.AccountID]struct{})
	for _, accountID := range accountIDs {
		m[accountID] = struct{}{}
	}
	require.Equal(t, len(m), len(accountIDs))
	require.True(t, len(m) > 100)
}
