package spam

import (
	_ "embed"
	rules "github.com/tonkeeper/scam_backoffice_rules"
)

type SpamFilter struct {
	Rules rules.Rules
}

func NewSpamFilter() *SpamFilter {
	s := &SpamFilter{
		Rules: rules.LoadRules(rules.GetDefaultRules(), true),
	}
	return s
}

func (s *SpamFilter) GetRules() rules.Rules {
	return s.Rules
}

func (s *SpamFilter) CheckAction(comment string) rules.TypeOfAction {
	action := rules.CheckAction(s.GetRules(), comment)
	return action
}
