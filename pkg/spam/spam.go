package spam

import (
	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo"
)

type SpamFilter struct {
	Rules          rules.Rules
	jettonVerifier *rules.JettonVerifier
}

func NewSpamFilter() *SpamFilter {
	return &SpamFilter{
		Rules:          rules.GetDefaultRules(),
		jettonVerifier: rules.NewJettonVerifier(),
	}
}

func (s *SpamFilter) GetRules() rules.Rules {
	return s.Rules
}

func (s *SpamFilter) IsJettonBlacklisted(address tongo.AccountID, symbol string) bool {
	return s.jettonVerifier.IsBlacklisted(address, symbol)
}
