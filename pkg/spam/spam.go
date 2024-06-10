package spam

import (
	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo"
)

type SpamFilter struct {
	Rules rules.Rules
}

func NewSpamFilter() *SpamFilter {
	return &SpamFilter{
		Rules: rules.GetDefaultRules(),
	}
}

func (s *SpamFilter) GetRules() rules.Rules {
	return s.Rules
}

func (s *SpamFilter) IsJettonBlacklisted(address tongo.AccountID, symbol string) bool {
	return false
}

func (s *SpamFilter) IsCollectionBlacklisted(address tongo.AccountID) bool {
	return false
}

func (s *SpamFilter) SpamDetector(amount int64, comment string) bool {
	return false
}
