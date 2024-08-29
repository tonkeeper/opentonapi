package spam

import (
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
)

type Filter struct {
	Rules rules.Rules
}

func NewSpamFilter() *Filter {
	return &Filter{
		Rules: rules.GetDefaultRules(),
	}
}

func (f Filter) CheckActions(actions []oas.Action, viewer *ton.AccountID) bool {
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

func (f Filter) NftTrust(address tongo.AccountID, collection *ton.AccountID, description, image string) core.TrustType {
	return core.TrustNone
}
