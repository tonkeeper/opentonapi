package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteapi"
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
				Status:            tlb.AccountActive,
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
			account, err := readFile[tlb.ShardAccount](tt.filename)
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

func TestConvertTransaction(t *testing.T) {
	tests := []struct {
		name           string
		accountID      tongo.AccountID
		txHash         tongo.Bits256
		txLt           uint64
		filenamePrefix string
		wantErr        bool
	}{
		{
			name:           "convert-tx-1",
			txHash:         tongo.MustParseHash("6c41096bbe0c2ca57f652ca7362a43473f8b33d8fa555a673bc70bb85fab37f6"),
			txLt:           37362820000003,
			accountID:      tongo.MustParseAccountID("0:6dcb8357c6bef52b43f0f681d976f5a46068ae195cb95f7a959d25c71b0cac6c"),
			filenamePrefix: "convert-tx-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
			require.Nil(t, err)
			txs, err := cli.GetTransactions(context.Background(), 1, tt.accountID, tt.txLt, tt.txHash)
			require.Nil(t, err)
			tx, err := ConvertTransaction(0, txs[0], nil)
			require.Nil(t, err)
			bs, err := json.MarshalIndent(tx, " ", "  ")
			require.Nil(t, err)
			outputName := fmt.Sprintf("testdata/%v.output.json", tt.filenamePrefix)
			if err := os.WriteFile(outputName, bs, 0644); err != nil {
				t.Fatalf("os.WriteFile() failed: %v", err)
			}
			expected, err := os.ReadFile(fmt.Sprintf("testdata/%v.json", tt.filenamePrefix))
			require.Nil(t, err)
			if bytes.Compare(bs, expected) != 0 {
				t.Fatalf("dont match")
			}
		})
	}
}
