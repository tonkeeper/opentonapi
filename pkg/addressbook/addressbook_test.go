package addressbook

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/cache"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
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

type mockPrivateBook struct {
	OnIsWallet func(a tongo.AccountID) (bool, error)
}

func (m *mockPrivateBook) IsWallet(a tongo.AccountID) (bool, error) {
	return m.OnIsWallet(a)
}

func (m *mockPrivateBook) GetAddress(a tongo.AccountID) (KnownAddress, bool) {
	panic("implement me")
}

func (m *mockPrivateBook) SearchAttachedAccounts(prefix string) []AttachedAccount {
	panic("implement me")
}

var _ addresser = (*mockPrivateBook)(nil)

func TestBook_IsWallet(t *testing.T) {
	tests := []struct {
		name         string
		addr         tongo.AccountID
		privateBooks []addresser
		wantIsWallet bool
		wantErr      bool
	}{
		{
			name:         "wallet from cache",
			addr:         ton.MustParseAccountID("0:6ccd325a858c379693fae2bcaab1c2906831a4e10a6c3bb44ee8b615bca1d220"),
			wantIsWallet: true,
		},
		{
			name:         "not a wallet from cache",
			addr:         ton.MustParseAccountID("0:779dcc815138d9500e449c5291e7f12738c23d575b5310000f6a253bd607384e"),
			wantIsWallet: false,
		},
		{
			name:         "wallet from private book",
			addr:         ton.MustParseAccountID("0:0000cc815138d9500e449c5291e7f12738c23d575b5310000f6a253bd607384e"),
			wantIsWallet: true,
			privateBooks: []addresser{
				&mockPrivateBook{
					OnIsWallet: func(a tongo.AccountID) (bool, error) {
						return true, nil
					},
				},
			},
		},
		{
			name:         "not a wallet from private book",
			addr:         ton.MustParseAccountID("0:0000cc815138d9500e449c5291e7f12738c23d575b5310000f6a253bd607384e"),
			wantIsWallet: false,
			privateBooks: []addresser{
				&mockPrivateBook{
					OnIsWallet: func(a tongo.AccountID) (bool, error) {
						return false, nil
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Book{
				walletsResolved: cache.NewLRUCache[tongo.AccountID, bool](100, "walletsResolved"),
				addressers:      tt.privateBooks,
			}
			b.walletsResolved.Set(ton.MustParseAccountID("0:6ccd325a858c379693fae2bcaab1c2906831a4e10a6c3bb44ee8b615bca1d220"), true)
			b.walletsResolved.Set(ton.MustParseAccountID("0:779dcc815138d9500e449c5291e7f12738c23d575b5310000f6a253bd607384e"), false)
			isWallet, err := b.IsWallet(tt.addr)
			require.Nil(t, err)
			require.Equal(t, tt.wantIsWallet, isWallet)

			isWallet, ok := b.walletsResolved.Get(tt.addr)
			require.True(t, ok)
			require.Equal(t, tt.wantIsWallet, isWallet)
		})
	}
}
