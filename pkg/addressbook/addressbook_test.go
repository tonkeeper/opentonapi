package addressbook

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/config"
	"github.com/tonkeeper/opentonapi/pkg/litestorage"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
)

func TestNormalizeString(t *testing.T) {
	type testCase struct {
		name   string
		input  string
		output string
	}
	tests := []testCase{
		{
			name:   "simple English sentence",
			input:  "The day that will always",
			output: "thedaythatwillalways",
		},
		{
			name:   "mixed case English words",
			input:  "TON Panda Baby",
			output: "tonpandababy",
		},
		{
			name:   "English with emoji at the end",
			input:  "apple 👎",
			output: "apple",
		},
		{
			name:   "emoji at the start",
			input:  "👎banana",
			output: "banana",
		},
		{
			name:   "uppercase Cyrillic",
			input:  "ХАЗЯЕВА",
			output: "хазяева",
		},
		{
			name:   "string with numbers and dots",
			input:  "11313.ton",
			output: "11313ton",
		},
		{
			name:   "Cyrillic sentence with spaces",
			input:  "Пока что думаю ",
			output: "покачтодумаю",
		},
		{
			name:   "English with multiple emojis",
			input:  "💎 TON Earth 🌍 Collectibles 🧭",
			output: "tonearthcollectibles",
		},
		{
			name:   "Cyrillic with numbers",
			input:  "Привет122",
			output: "привет122",
		},
		{
			name:   "string with numbers and domain-like structure",
			input:  "11313.t.me",
			output: "11313tme",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			convertedData := strings.ToLower(NormalizeReg.ReplaceAllString(test.input, ""))
			require.Equal(t, test.output, convertedData)
		})
	}
}

func TestSearchAttachedAccountsByPrefix(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
	require.NoError(t, err, "Failed to create lite API client")

	liteStorage, err := litestorage.NewLiteStorage(logger, client)
	require.NoError(t, err, "Failed to create lite storage")

	book := NewAddressBook(logger, config.AddressPath, config.JettonPath, config.CollectionPath, liteStorage)

	// Waiting background processes
	time.Sleep(time.Second)

	tests := []struct {
		name     string
		request  string
		expected *AttachedAccount
	}{
		{
			name:    "Exact match with full name",
			request: "TON Believers Fund",
			expected: &AttachedAccount{
				Name:   "TON Believers Fund",
				Wallet: ton.MustParseAccountID("0:ed1691307050047117b998b561d8de82d31fbf84910ced6eb5fc92e7485ef8a7"),
			},
		},
		{
			name:    "Partial match",
			request: "believers fund",
			expected: &AttachedAccount{
				Name:   "TON Believers Fund",
				Wallet: ton.MustParseAccountID("0:ed1691307050047117b998b561d8de82d31fbf84910ced6eb5fc92e7485ef8a7"),
			},
		},
		{
			name:    "Single word partial match",
			request: "fund",
			expected: &AttachedAccount{
				Name:   "TON Believers Fund",
				Wallet: ton.MustParseAccountID("0:ed1691307050047117b998b561d8de82d31fbf84910ced6eb5fc92e7485ef8a7"),
			},
		},
		{
			name:     "No match",
			request:  "random string",
			expected: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			accounts := book.SearchAttachedAccountsByPrefix(test.request)
			if test.expected == nil {
				require.Empty(t, accounts, "Expected no results, but got: %v", len(accounts))
				return
			}
			// Check if the expected account is in the result
			var found bool
			for _, account := range accounts {
				if account.Wallet == test.expected.Wallet {
					found = true
					break
				}
			}
			require.True(t, found, "Expected account not found in the results: %v", test.expected)
		})
	}
}

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
