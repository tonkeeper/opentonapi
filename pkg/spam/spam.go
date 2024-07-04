package spam

import (
	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/core"
	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
)

type Filter struct {
	Rules rules.Rules
}

func (f Filter) CheckActions(actions []bath.Action, viewer *ton.AccountID) bool {
	var comment string
	for _, action := range actions {
		switch {
		case action.TonTransfer != nil && action.TonTransfer.Comment != nil:
			comment = *action.TonTransfer.Comment
		case action.JettonTransfer != nil && action.JettonTransfer.Comment != nil:
			comment = *action.JettonTransfer.Comment
		case action.NftItemTransfer != nil && action.NftItemTransfer.Comment != nil:
			comment = *action.NftItemTransfer.Comment
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

func (f Filter) NftTrust(address tongo.AccountID, collection *ton.AccountID, description, image string, isApproved bool) core.TrustType {
	return core.TrustNone
}

func NewSpamFilter() *Filter {
	return &Filter{
		Rules: rules.GetDefaultRules(),
	}
}
