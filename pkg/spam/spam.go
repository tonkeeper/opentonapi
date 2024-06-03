package spam

import (
	"sync"

	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo"
)

type SpamFilter struct {
	mu                     sync.RWMutex
	Rules                  rules.Rules
	blacklistedCollections map[tongo.AccountID]bool
	jettonVerifier         *rules.JettonVerifier
}

func NewSpamFilter() *SpamFilter {
	return &SpamFilter{
		Rules:                  rules.GetDefaultRules(),
		jettonVerifier:         rules.NewJettonVerifier(),
		blacklistedCollections: make(map[tongo.AccountID]bool),
	}
}

func (s *SpamFilter) GetRules() rules.Rules {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Rules
}

func (s *SpamFilter) IsJettonBlacklisted(address tongo.AccountID, symbol string) bool {
	return s.jettonVerifier.IsBlacklisted(address, symbol)
}

func (s *SpamFilter) IsCollectionBlacklisted(address tongo.AccountID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.blacklistedCollections[address]
}
