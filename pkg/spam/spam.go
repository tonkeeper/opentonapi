package spam

import (
	"context"
	"net/url"
	"strings"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
)

// blacklistedImageHosts lists domains that, when hosting an NFT image, mark the
// NFT (or collection) as scam.
var blacklistedImageHosts = []string{
	"cloudmetrics.cyou",
}

// imageHostBlacklisted reports whether the given image URL is hosted under one
// of the blacklisted domains (the host itself or any of its subdomains).
func imageHostBlacklisted(image string) bool {
	u, err := url.Parse(strings.TrimSpace(image))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	for _, blacklisted := range blacklistedImageHosts {
		if host == blacklisted || strings.HasSuffix(host, "."+blacklisted) {
			return true
		}
	}
	return false
}

type Filter struct {
	Rules rules.Rules
}

func NewSpamFilter() *Filter {
	return &Filter{
		Rules: rules.GetDefaultRules(),
	}
}

func (f *Filter) GetNftsScamData(ctx context.Context, addresses []ton.AccountID) (map[ton.AccountID]core.TrustType, error) {
	return nil, nil
}

func (f *Filter) GetEventsScamData(ctx context.Context, ids []string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (f Filter) IsScamEvent(actions []oas.Action, viewer *ton.AccountID, initiator ton.AccountID) bool {
	var comment string
	for _, action := range actions {
		switch {
		case action.TonTransfer.IsSet():
			comment = action.TonTransfer.Value.Comment.Value
		case action.JettonTransfer.IsSet():
			comment = action.JettonTransfer.Value.Comment.Value
		case action.NftItemTransfer.IsSet():
			comment = action.NftItemTransfer.Value.Comment.Value
		default:
			continue
		}
		for _, rule := range f.Rules {
			rate := rule.Evaluate(comment)
			if rate == rules.Drop || rate == rules.MarkScam {
				return true
			}

		}
	}
	return false
}

func (f Filter) JettonTrust(address tongo.AccountID, symbol, name, image string) core.TrustType {
	return core.TrustNone
}

func (f Filter) NftTrust(address tongo.AccountID, collection, owner, collectionOwner *ton.AccountID, name, description, image, collectionName, collectionDescription string) core.TrustType {
	if imageHostBlacklisted(image) {
		return core.TrustBlacklist
	}
	return core.TrustNone
}

func (f Filter) AccountTrust(address tongo.AccountID) core.TrustType {
	return core.TrustNone
}

func (f Filter) HasBlacklistedComment(values ...string) bool {
	return false
}

func (f Filter) TonDomainTrust(domain string) core.TrustType {
	return core.TrustNone
}
