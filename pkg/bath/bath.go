package bath

import (
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
)

type ActionsList struct {
	Actions   []Action
	ValueFlow *ValueFlow
}

type addressBook interface {
	GetJettonInfoByAddress(a tongo.AccountID) (addressbook.KnownJetton, bool)
}

type Options struct {
	straws      []Straw
	account     *tongo.AccountID
	addressBook addressBook
}

type Option func(*Options)

// WithStraws provides functions to find actions in a trace.
func WithStraws(straws []Straw) Option {
	return func(options *Options) {
		options.straws = straws
	}
}

func ForAccount(a tongo.AccountID) Option {
	return func(options *Options) {
		options.account = &a
	}
}

func WithAddressbook(book addressBook) Option {
	return func(options *Options) {
		options.addressBook = book
	}
}

// FindActions finds known action patterns in the given trace and
// returns a list of actions.
func FindActions(trace *core.Trace, opts ...Option) (*ActionsList, error) {
	options := Options{
		straws: DefaultStraws,
	}
	for _, o := range opts {
		o(&options)
	}
	bubble := fromTrace(trace)
	MergeAllBubbles(bubble, options.straws)
	actions, flow := CollectActionsAndValueFlow(bubble, options.account, options.addressBook)
	return &ActionsList{
		Actions:   actions,
		ValueFlow: flow,
	}, nil
}

func MergeAllBubbles(bubble *Bubble, straws []Straw) {
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

func recursiveMerge(bubble *Bubble, s Straw) bool {
	if s(bubble) {
		return true
	}
	for _, b := range bubble.Children {
		if recursiveMerge(b, s) {
			return true
		}
	}
	return false
}
