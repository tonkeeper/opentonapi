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
	return nil
}

func (s *NoOpSpamFilter) IsJettonBlacklisted(address tongo.AccountID, symbol string) bool {
	return false
}
