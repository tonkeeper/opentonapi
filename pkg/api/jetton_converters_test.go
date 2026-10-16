package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tep64"
	"github.com/tonkeeper/tongo/ton"
)

// metadatalessStorage knows neither scaled UI parameters nor any master's metadata. Every other method
// is left nil on purpose.
type metadatalessStorage struct {
	scaledlessStorage
}

func (metadatalessStorage) GetJettonMasterMetadata(ctx context.Context, master tongo.AccountID) (tep64.Metadata, error) {
	return tep64.Metadata{}, core.ErrEntityNotFound
}

// noRates has no rate for any date. Every other method is left nil on purpose.
type noRates struct {
	ratesSource
}

func (noRates) GetRates(date int64) (map[string]float64, error) {
	return map[string]float64{}, nil
}

func TestConvertJettonBalanceCarriesTheWalletsLt(t *testing.T) {
	wallet := ton.MustParseAccountID("0:fc35e51a7890d259858bd1caab05052a942006d38e95bee1d8b5045174b63653")
	master := ton.MustParseAccountID("0:5a01bc2497790051d28ccde06c09b2a43e205d8d089370bf0490bbba093cc5af")
	h := &Handler{
		storage: metadatalessStorage{},
		addressBook: mockAddressBook{
			OnGetAddressInfoByAddress: func(a tongo.AccountID) (addressbook.KnownAddress, bool) {
				return addressbook.KnownAddress{}, false
			},
		},
		spamFilter:  mockSpamFilter{},
		score:       zeroScore{},
		ratesSource: noRates{},
	}
	lt := int64(25713146000001)
	for _, tt := range []struct {
		name string
		lt   *int64
	}{
		{name: "an indexed wallet", lt: &lt},
		{name: "an unclaimed mintless balance", lt: nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := h.convertJettonBalance(context.Background(), core.JettonWallet{Address: wallet, JettonAddress: master, BalanceLt: tt.lt}, nil, nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.lt != nil, got.BalanceLt.Set)
			if tt.lt != nil {
				assert.Equal(t, *tt.lt, got.BalanceLt.Value)
			}
		})
	}
}
