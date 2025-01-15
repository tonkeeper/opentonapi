package addressbook

import (
	"reflect"
	"sort"
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

func TestGenerateNameVariants(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Single word",
			input:    "TON",
			expected: []string{"TON"},
		},
		{
			name:     "Two words",
			input:    "TON Foundation",
			expected: []string{"TON Foundation", "Foundation TON"},
		},
		{
			name:     "Three words",
			input:    "TON Believers Fund",
			expected: []string{"TON Believers Fund", "Believers Fund TON", "Fund TON Believers"},
		},
		{
			name:     "Four words",
			input:    "Notcoin Foundation of Believers",
			expected: []string{"Notcoin Foundation of Believers", "Foundation of Believers Notcoin", "of Believers Notcoin Foundation"},
		},
		{
			name:     "Five words",
			input:    "The Open Network Blockchain Project",
			expected: []string{"The Open Network Blockchain Project", "Open Network Blockchain Project The", "Network Blockchain Project The Open"},
		},
		{
			name:     "Six words",
			input:    "The First Decentralized Cryptocurrency of TON",
			expected: []string{"The First Decentralized Cryptocurrency of TON", "First Decentralized Cryptocurrency of TON The", "Decentralized Cryptocurrency of TON The First"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateSlugVariants(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, but got %v", tt.expected, result)
			}
		})
	}
}

func TestFindIndexes(t *testing.T) {
	tests := []struct {
		name       string
		accounts   []AttachedAccount
		prefix     string
		startIndex int
		endIndex   int
	}{
		{
			name: "Prefix found",
			accounts: []AttachedAccount{
				{Normalized: "toncoin"},
				{Normalized: "tonstarter"},
				{Normalized: "tonstarter"},
				{Normalized: "tonswap"},
				{Normalized: "uniswap"},
			},
			prefix:     "tonst",
			startIndex: 1,
			endIndex:   2,
		},
		{
			name: "Prefix not found",
			accounts: []AttachedAccount{
				{Normalized: "toncoin"},
				{Normalized: "tonstarter"},
				{Normalized: "tonswap"},
				{Normalized: "uniswap"},
			},
			prefix:     "xyz",
			startIndex: -1,
			endIndex:   -1,
		},
		{
			name: "Prefix at start",
			accounts: []AttachedAccount{
				{Normalized: "toncoin"},
				{Normalized: "tonstarter"},
				{Normalized: "tonswap"},
				{Normalized: "uniswap"},
			},
			prefix:     "tonco",
			startIndex: 0,
			endIndex:   0,
		},
		{
			name: "Prefix at end",
			accounts: []AttachedAccount{
				{Normalized: "toncoin"},
				{Normalized: "tonstarter"},
				{Normalized: "tonswap"},
				{Normalized: "uniswap"},
			},
			prefix:     "uniswap",
			startIndex: 3,
			endIndex:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sort.Slice(tt.accounts, func(i, j int) bool {
				return tt.accounts[i].Normalized < tt.accounts[j].Normalized
			})
			startIdx, endIdx := FindIndexes(tt.accounts, tt.prefix)
			if startIdx != tt.startIndex || endIdx != tt.endIndex {
				t.Errorf("Expected (%d, %d), but got (%d, %d)", tt.startIndex, tt.endIndex, startIdx, endIdx)
			}
		})
	}
}

func TestSearchAccountByName(t *testing.T) {
	type testCase struct {
		name   string
		book   *Book
		query  string
		result []string
	}
	for _, test := range []testCase{
		{
			name: "Search by first letters with one found account",
			book: &Book{addressers: []addresser{&manualAddresser{
				sorted: []AttachedAccount{
					{
						Name:       "elon-musk.ton",
						Normalized: "elonmuskton",
					},
				},
			}}},
			query:  "elon",
			result: []string{"elonmuskton"},
		},
		{
			name: "Search by full name with one found account",
			book: &Book{addressers: []addresser{&manualAddresser{
				sorted: []AttachedAccount{
					{
						Name:       "elon-musk.ton",
						Normalized: "elonmuskton",
					},
				},
			}}},
			query:  "elon-musk.ton",
			result: []string{"elonmuskton"},
		},
		{
			name: "Search by first letters with multiple accounts found",
			book: &Book{addressers: []addresser{&manualAddresser{
				sorted: []AttachedAccount{
					{
						Name:       "elongate-blah.ton",
						Wallet:     ton.MustParseAccountID("Ef9VVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVbxn"),
						Normalized: "elongateblahton",
					},
					{
						Name:       "elongate.ton",
						Wallet:     ton.MustParseAccountID("Ef8zMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzM0vF"),
						Normalized: "elongateton",
					},
					{
						Name:       "elon-musk.ton",
						Wallet:     ton.MustParseAccountID("Ef_lZ1T4NCb2mwkme9h2rJfESCE0W34ma9lWp7-_uY3zXDvq"),
						Normalized: "elonmuskton",
					},
				},
			}}},
			query:  "elon",
			result: []string{"elongateblahton", "elongateton", "elonmuskton"},
		},
		{
			name: "Search through an empty list",
			book: &Book{addressers: []addresser{&manualAddresser{
				sorted: []AttachedAccount{}},
			}},
			query:  "blah",
			result: []string{},
		},
		{
			name: "Search for a word that doesn't exist",
			book: &Book{addressers: []addresser{&manualAddresser{
				sorted: []AttachedAccount{
					{
						Name:       "elon-musk.ton",
						Normalized: "elonmuskton",
					},
				},
			}}},
			query:  "blah",
			result: []string{},
		},
		{
			name: "Search with case insensitive query",
			book: &Book{addressers: []addresser{&manualAddresser{
				sorted: []AttachedAccount{
					{
						Name:       "Elon-Musk.ton",
						Normalized: "elonmuskton",
					},
				},
			}}},
			query:  "ELON",
			result: []string{"elonmuskton"},
		},
		{
			name: "Search with empty query string",
			book: &Book{addressers: []addresser{&manualAddresser{
				sorted: []AttachedAccount{
					{
						Name:       "elon-musk.ton",
						Normalized: "elonmuskton",
					},
				},
			}}},
			query:  "",
			result: []string{},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var foundAccounts []string
			attachedAccounts := test.book.SearchAttachedAccountsByPrefix(test.query)
			for _, account := range attachedAccounts {
				foundAccounts = append(foundAccounts, account.Normalized)
			}
			require.ElementsMatch(t, foundAccounts, test.result)
		})
	}
}
