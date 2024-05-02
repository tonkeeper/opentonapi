package bath

import (
	"context"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
)

type ActionsList struct {
	Actions   []Action
	ValueFlow *ValueFlow
}

type Options struct {
	straws            []Merger
	account           *tongo.AccountID
	informationSource core.InformationSource
}

type Option func(*Options)

// WithStraws provides functions to find actions in a trace.
func WithStraws(straws []Merger) Option {
	return func(options *Options) {
		options.straws = straws
	}
}

func ForAccount(a tongo.AccountID) Option {
	return func(options *Options) {
		options.account = &a
	}
}

func WithInformationSource(source core.InformationSource) Option {
	return func(options *Options) {
		options.informationSource = source
	}
}

// FindActions finds known action patterns in the given trace and
// returns a list of actions.
func FindActions(ctx context.Context, trace *core.Trace, opts ...Option) (*ActionsList, error) {
	options := Options{
		straws: DefaultStraws,
	}
	for _, o := range opts {
		o(&options)
	}
	if err := core.CollectAdditionalInfo(ctx, options.informationSource, trace); err != nil {
		return nil, err
	}
	bubble := fromTrace(trace)
	MergeAllBubbles(bubble, options.straws)
	actions, flow := CollectActionsAndValueFlow(bubble, options.account)
	return &ActionsList{
		Actions:   actions,
		ValueFlow: flow,
	}, nil
}

func MergeAllBubbles(bubble *Bubble, straws []Merger) {
	for _, s := range straws {
		for {
			success := recursiveMerge(bubble, s)
			if success {
				continue
			}
			break
		}
	}
}

func recursiveMerge(bubble *Bubble, s Merger) bool {
	if s.Merge(bubble) {
		return true
	}
	for _, b := range bubble.Children {
		if recursiveMerge(b, s) {
			return true
		}
	}
	return false
}

func (l *ActionsList) Extra(account tongo.AccountID) int64 {
	extra := int64(0)
	if flow, ok := l.ValueFlow.Accounts[account]; ok {
		extra += flow.Ton
	}
	for _, action := range l.Actions {
		extra -= action.ContributeToExtra(account)
	}
	return extra
}
