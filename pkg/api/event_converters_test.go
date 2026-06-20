package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
)

// mockSpamFilter implements the SpamFilter interface for converter tests.
// Only AccountTrust is meaningful; everything else is a no-op.
type mockSpamFilter struct {
	blacklist map[ton.AccountID]bool
}

func (m mockSpamFilter) IsScamEvent(actions []oas.Action, viewer *ton.AccountID, initiator ton.AccountID) bool {
	return false
}

func (m mockSpamFilter) GetEventsScamData(ctx context.Context, ids []string) (map[string]bool, error) {
	return nil, nil
}

func (m mockSpamFilter) JettonTrust(address tongo.AccountID, symbol, name, image string) core.TrustType {
	return core.TrustNone
}

func (m mockSpamFilter) AccountTrust(address tongo.AccountID) core.TrustType {
	if m.blacklist[address] {
		return core.TrustBlacklist
	}
	return core.TrustNone
}

func (m mockSpamFilter) HasBlacklistedComment(values ...string) bool {
	return false
}

func (m mockSpamFilter) TonDomainTrust(domain string) core.TrustType {
	return core.TrustNone
}

func (m mockSpamFilter) NftTrust(address tongo.AccountID, collection, owner *ton.AccountID, name, description, image, collectionName, collectionDescription string) core.TrustType {
	return core.TrustNone
}

func (m mockSpamFilter) GetNftsScamData(ctx context.Context, addresses []ton.AccountID) (map[ton.AccountID]core.TrustType, error) {
	return nil, nil
}

var _ SpamFilter = mockSpamFilter{}

func TestConvertTraceScamPropagation(t *testing.T) {
	acc := func(b byte) ton.AccountID {
		a := ton.AccountID{Workchain: 0}
		a.Address[0] = b
		return a
	}
	scammer := acc(1)
	victim := acc(2)
	downstream := acc(3)
	clean := acc(4)

	book := mockAddressBook{
		OnGetAddressInfoByAddress: func(a tongo.AccountID) (addressbook.KnownAddress, bool) {
			return addressbook.KnownAddress{}, false
		},
	}

	// root -> victim (sent by root) -> downstream (sent by victim)
	makeTree := func(root, victimSource ton.AccountID) *core.Trace {
		return &core.Trace{
			Transaction: core.Transaction{
				TransactionID: core.TransactionID{Account: root, Lt: 1},
			},
			Children: []*core.Trace{
				{
					Transaction: core.Transaction{
						TransactionID: core.TransactionID{Account: victim, Lt: 2},
						InMsg:         &core.Message{MessageID: core.MessageID{CreatedLt: 2, Source: &victimSource}},
					},
					Children: []*core.Trace{
						{
							Transaction: core.Transaction{
								TransactionID: core.TransactionID{Account: downstream, Lt: 3},
								InMsg:         &core.Message{MessageID: core.MessageID{CreatedLt: 3, Source: &victim}},
							},
						},
					},
				},
			},
		}
	}

	h := &Handler{spamFilter: mockSpamFilter{blacklist: map[ton.AccountID]bool{scammer: true}}}

	t.Run("scam origin flags the whole tree", func(t *testing.T) {
		got := h.convertTrace(makeTree(scammer, scammer), book)
		assert.True(t, got.Transaction.Account.IsScam)
		assert.True(t, got.Children[0].Transaction.Account.IsScam)
		assert.True(t, got.Children[0].Children[0].Transaction.Account.IsScam)
	})

	t.Run("clean origin flags nothing", func(t *testing.T) {
		got := h.convertTrace(makeTree(clean, clean), book)
		assert.False(t, got.Transaction.Account.IsScam)
		assert.False(t, got.Children[0].Transaction.Account.IsScam)
		assert.False(t, got.Children[0].Children[0].Transaction.Account.IsScam)
	})

	t.Run("blacklisted mid-tree sender flags its subtree only", func(t *testing.T) {
		// Clean root, but the child transaction is sent by the scammer: the child
		// and everything below it is scam, the root is not.
		got := h.convertTrace(makeTree(clean, scammer), book)
		assert.False(t, got.Transaction.Account.IsScam)
		assert.True(t, got.Children[0].Transaction.Account.IsScam)
		assert.True(t, got.Children[0].Children[0].Transaction.Account.IsScam)
	})
}
