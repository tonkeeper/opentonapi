package bath

import (
	"context"
	"fmt"

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
	bubble := fromTrace(trace, nil)
	MergeAllBubbles(bubble, options.straws)
	actions, flow := CollectActionsAndValueFlow(bubble, options.account)
	return &ActionsList{
		Actions:   actions,
		ValueFlow: flow,
	}, nil
}

func MergeAllBubbles(bubble *Bubble, straws []Merger) {
	for i, s := range straws {
		for {
			success := recursiveMerge(bubble, s, i)
			if success {
				continue
			}
			break
		}
	}
}

func recursiveMerge(bubble *Bubble, s Merger, idx int) bool {
	if s.Merge(bubble) {
		strawSuccess.WithLabelValues(fmt.Sprintf("%d", idx)).Inc()
		return true
	}
	for _, b := range bubble.Children {
		if recursiveMerge(b, s, idx) {
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
