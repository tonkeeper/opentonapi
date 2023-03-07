package core

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
)

func readFile[T any](filename string) (*T, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var t T
	if err := json.Unmarshal(bytes, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func TestConvertToAccount(t *testing.T) {
	tests := []struct {
		name            string
		accountID       tongo.AccountID
		filename        string
		want            *Account
		wantCodePresent bool
		wantDataPresent bool
	}{
		{
			name:      "active account with data",
			filename:  "testdata/account.json",
			accountID: tongo.MustParseAccountID("EQDendoireMDFMufOUzkqNpFIay83GnjV2tgGMbA64wA3siV"),
			want: &Account{
				AccountAddress:    tongo.MustParseAccountID("EQDendoireMDFMufOUzkqNpFIay83GnjV2tgGMbA64wA3siV"),
				Status:            "active",
				TonBalance:        989109352,
				ExtraBalances:     nil,
				LastTransactionLt: 31236013000006,
				Storage: StorageInfo{
					UsedCells:       *big.NewInt(46),
					UsedBits:        *big.NewInt(13485),
					UsedPublicCells: *big.NewInt(0),
					LastPaid:        1663270333,
					DuePayment:      0,
				},
			},
			wantDataPresent: true,
			wantCodePresent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account, err := readFile[tlb.Account](tt.filename)
			require.Nil(t, err)
			got, err := ConvertToAccount(tt.accountID, *account)
			require.Nil(t, err)
			if tt.wantCodePresent {
				require.True(t, len(got.Code) > 0)
			} else {
				require.Nil(t, got)
			}
			if tt.wantDataPresent {
				require.True(t, len(got.Data) > 0)
			} else {
				require.Nil(t, got)
			}
			got.Code = nil
			got.Data = nil
			require.Equal(t, tt.want, got)
		})
	}
}
