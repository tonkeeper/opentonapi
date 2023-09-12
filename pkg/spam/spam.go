package spam

import (
	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo"
)

type SpamFilter struct {
	Rules          rules.Rules
	JettonEvaluate *rules.JettonEvaluate
}

func NewSpamFilter() *SpamFilter {
	return &SpamFilter{
		Rules:          rules.GetDefaultRules(),
		JettonEvaluate: rules.NewJettonEvaluate(),
	}
}

func (s *SpamFilter) GetRules() rules.Rules {
	return s.Rules
}

func (s *SpamFilter) CheckJettonAction(address tongo.AccountID, symbol string) rules.TypeOfAction {
	return s.JettonEvaluate.SearchAction(address, symbol)
}
