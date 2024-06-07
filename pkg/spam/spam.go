package spam

import (
	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo/ton"
)

type CollectionVerificationType string

const (
	CollectionVerificationTypeBlacklist CollectionVerificationType = "blacklist"
	CollectionVerificationTypeGraylist  CollectionVerificationType = "graylist"
	CollectionVerificationTypeNone      CollectionVerificationType = "none"
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

func (s *SpamFilter) IsJettonBlacklisted(address ton.AccountID, symbol string) bool {
	return false
}

func (s *SpamFilter) GetBlacklistedDomains() []string {
	return []string{}
}

func (s *SpamFilter) GetCollectionVerificationType(address ton.AccountID) CollectionVerificationType {
	return CollectionVerificationTypeNone
}
