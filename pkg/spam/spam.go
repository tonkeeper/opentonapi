package spam

import (
	_ "embed"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo"
)

type Spam struct {
	Rules rules.Rules
}

//go:embed rules.yaml
var bytesOfRules []byte

func NewSpam() *Spam {
	s := &Spam{
		Rules: rules.LoadRules(bytesOfRules, true),
	}
	return s
}

func (s *Spam) GetRules() rules.Rules {
	return s.Rules
}

func MarkScamEvent(event oas.AccountEvent, spamRules rules.Rules) oas.AccountEvent {
	for i, a := range event.Actions {
		if a.Type != oas.ActionTypeTonTransfer || a.TonTransfer.Value.Comment.Value == "" || a.TonTransfer.Value.Amount > int64(tongo.OneTON) {
			continue
		}
		action := rules.CheckAction(spamRules, a.TonTransfer.Value.Comment.Value)
		if action == rules.Drop {
			event.IsScam = true
			event.Actions[i].TonTransfer.Value.Comment.Reset()
			continue
		}
	}
	return event
}
