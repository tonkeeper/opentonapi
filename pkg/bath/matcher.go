package bath

import (
	"github.com/tonkeeper/opentonapi/pkg/sentry"
	"github.com/tonkeeper/tongo/abi"
)

type bubbleCheck func(bubble *Bubble) bool
type Straw[newBubbleT actioner] struct {
	CheckFuncs []bubbleCheck
	Builder    func(newAction *newBubbleT, bubble *Bubble) error
	Children   []Straw[newBubbleT]
	Optional   bool
}

//todo: https://tonviewer.com/transaction/16462168398a5c6324602beb1da2e90ab5510aaf180ec00404620a33487fa180

func (s Straw[newBubbleT]) match(mapping map[*Bubble]Straw[newBubbleT], bubble *Bubble) bool {
	for _, checkFunc := range s.CheckFuncs {
		if !checkFunc(bubble) {
			return false
		}
	}
	for _, childStraw := range s.Children {
		found := false
		for _, child := range bubble.Children {
			if childStraw.match(mapping, child) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if _, ok := mapping[bubble]; ok {
		//todo: log or maybe return error
		return false
	}

	mapping[bubble] = s
	return true
}

func (s Straw[newBubbleT]) Merge(bubble *Bubble) bool {
	mapping := make(map[*Bubble]Straw[newBubbleT])
	if !s.match(mapping, bubble) {
		return false
	}
	var newBubble newBubbleT
	var newChildren []*Bubble
	newAccounts := bubble.Accounts
	nvf := newValueFlow()
	nvf.Merge(bubble.ValueFlow)
	for b, straw := range mapping {
		if straw.Builder != nil {
			err := straw.Builder(&newBubble, b)
			if err != nil {
				sentry.Send("Straw.Merge", sentry.SentryInfoData{"error": err.Error(), "bubble": b.String()}, sentry.LevelError)
				return false
			}
		}
		nvf.Merge(b.ValueFlow)
		newAccounts = append(newAccounts, b.Accounts...)
		for _, child := range b.Children {
			if _, ok := mapping[child]; ok {
				continue
			}
			newChildren = append(newChildren, child)
		}
	}
	n := Bubble{
		Info:      newBubble,
		Accounts:  newAccounts,
		Children:  newChildren,
		ValueFlow: nvf,
	}
	*bubble = n
	return true
}

func IsTx(b *Bubble) bool {
	_, ok := b.Info.(BubbleTx)
	return ok
}

func Or(check1, check2 bubbleCheck) bubbleCheck {
	return func(bubble *Bubble) bool {
		return check1(bubble) || check2(bubble)
	}
}

func HasOpcode(op uint32) bubbleCheck {
	return func(b *Bubble) bool {
		opCode := b.Info.(BubbleTx).opCode
		return opCode != nil && *opCode == op
	}
}

func HasOperation(name abi.MsgOpName) bubbleCheck {
	return func(b *Bubble) bool {
		return b.Info.(BubbleTx).operation(name)
	}
}
