package spam

import (
	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo"
)

// NoOpSpamFilter is a spam filter that does nothing and pretends there is no spam in the TON blockchain.
type NoOpSpamFilter struct {
}

func NewNoOpSpamFilter() *NoOpSpamFilter {
	return &NoOpSpamFilter{}
}

func (s *NoOpSpamFilter) GetRules() rules.Rules {
	return rules.Rules{}
}

func (s *NoOpSpamFilter) GetBlacklistedCollections() map[tongo.AccountID]bool {
	return map[tongo.AccountID]bool{}
}

func (s *NoOpSpamFilter) IsJettonBlacklisted(address tongo.AccountID, symbol string) bool {
	return false
}

func (s *NoOpSpamFilter) IsCollectionBlacklisted(address tongo.AccountID) bool {
	return false
}
