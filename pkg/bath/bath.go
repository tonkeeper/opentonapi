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
	Straws      []Straw
	AddressBook addressBook
}

type Option func(*Options)

// WithStraws provides functions to find actions in a trace.
func WithStraws(straws []Straw) Option {
	return func(options *Options) {
		options.Straws = straws
	}
}

func WithAddressbook(book addressBook) Option {
	return func(options *Options) {
		options.AddressBook = book
	}
}

// FindActions finds known action patterns in the given trace and
// returns a list of actions.
func FindActions(trace *core.Trace, opts ...Option) (*ActionsList, error) {
	options := Options{
		Straws: DefaultStraws,
	}
	for _, o := range opts {
		o(&options)
	}
	bubble := fromTrace(trace, nil)
	MergeAllBubbles(bubble, options.Straws)
	actions, flow := CollectActionsAndValueFlow(bubble, nil, options.AddressBook)
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
