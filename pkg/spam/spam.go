package spam

import (
	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo/ton"
)

type VerificationType string

const (
	VerificationTypeBlacklist VerificationType = "blacklist"
	VerificationTypeGraylist  VerificationType = "graylist"
	VerificationTypeNone      VerificationType = "none"
)

type Filter struct {
	Rules rules.Rules
}

func NewSpamFilter() *Filter {
	return &Filter{
		Rules: rules.GetDefaultRules(),
	}
}

func (s *Filter) GetRules() rules.Rules {
	return s.Rules
}

func (s *Filter) IsJettonBlacklisted(address ton.AccountID, symbol string) bool {
	return false
}

func (s *Filter) GetBlacklistedDomains() []string {
	return []string{}
}

func (s *Filter) GetVerificationType(address ton.AccountID) VerificationType {
	return VerificationTypeNone
}

func (s *SpamFilter) SpamDetector(amount int64, comment string) bool {
	return false
}
