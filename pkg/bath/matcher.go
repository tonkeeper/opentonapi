package bath

import (
	"unsafe"

	"github.com/tonkeeper/opentonapi/pkg/sentry"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"golang.org/x/exp/slices"
)

type bubbleCheck func(bubble *Bubble) bool
type Straw[newBubbleT actioner] struct {
	CheckFuncs       []bubbleCheck
	Builder          func(newAction *newBubbleT, bubble *Bubble) error //uses to convert old bubble to new Bubble.Info
	ValueFlowUpdater func(newAction *newBubbleT, flow *ValueFlow)
	SingleChild      *Straw[newBubbleT]
	Children         []Straw[newBubbleT]
	Optional         bool
}

func (s Straw[newBubbleT]) match(bubble *Bubble) (mappings []struct {
	s Straw[newBubbleT]
	b *Bubble
}) {
	for _, checkFunc := range s.CheckFuncs {
		if !checkFunc(bubble) {
			return nil
		}
	}
	if s.SingleChild != nil && len(s.Children) != 0 {
		panic("Straw can't have both SingleChild and Children")
	}
	if s.SingleChild != nil {
		found := false
		for _, child := range bubble.Children {
			m := s.SingleChild.match(child)
			if len(m) != 0 {
				found = true
				mappings = append(mappings, m...)
				break
			}
		}
		if !(found || s.SingleChild.Optional) {
			return nil
		}
	}
	for _, childStraw := range s.Children {
		found := false
		for _, child := range bubble.Children {
			m := childStraw.match(child)
			if len(m) != 0 {
				found = true
				mappings = append(mappings, m...)
				break
			}
		}
		if !(found || childStraw.Optional) {
			return nil
		}
	}
	mappings = append(mappings, struct {
		s Straw[newBubbleT]
		b *Bubble
	}{s, bubble})
	return mappings
}

func (s Straw[newBubbleT]) Merge(bubble *Bubble) bool {
	mapping := s.match(bubble)
	if len(mapping) == 0 {
		return false
	}
	var newBubble newBubbleT
	var newChildren []*Bubble
	newAccounts := bubble.Accounts
	newTransaction := bubble.Transaction
	nvf := newValueFlow()
	var finalizer func(newAction *newBubbleT, flow *ValueFlow)
	for i := len(mapping) - 1; i >= 0; i-- {
		if mapping[i].s.Builder != nil {
			err := mapping[i].s.Builder(&newBubble, mapping[i].b)
			if err != nil {
				sentry.Send("Straw.Merge", sentry.SentryInfoData{"error": err.Error(), "bubble": mapping[i].b.String()}, sentry.LevelError)
				return false
			}
		}
		if mapping[i].s.ValueFlowUpdater != nil {
			finalizer = mapping[i].s.ValueFlowUpdater
		}
		nvf.Merge(mapping[i].b.ValueFlow)
		newAccounts = append(newAccounts, mapping[i].b.Accounts...)
		newTransaction = append(newTransaction, mapping[i].b.Transaction...)
		for _, child := range mapping[i].b.Children {
			if slices.ContainsFunc(mapping, func(s struct {
				s Straw[newBubbleT]
				b *Bubble
			}) bool {
				return s.b == child
			}) {
				continue
			}
			newChildren = append(newChildren, child)
		}
	}
	if finalizer != nil {
		finalizer(&newBubble, nvf)
	}
	n := Bubble{
		Info:        newBubble,
		Accounts:    newAccounts,
		Children:    newChildren,
		ValueFlow:   nvf,
		Transaction: newTransaction,
	}
	*bubble = n
	return true
}

// Optional returns copy of declarative straw but optional
func Optional[T actioner](s Straw[T]) Straw[T] {
	s.Optional = true
	return s
}

func IsTx(b *Bubble) bool {
	_, ok := b.Info.(BubbleTx)
	return ok
}

type interfaceType struct {
	typ  unsafe.Pointer
	word unsafe.Pointer
}

func Is(t actioner) bubbleCheck {
	return func(bubble *Bubble) bool {
		return (*interfaceType)(unsafe.Pointer(&t)).typ == (*interfaceType)(unsafe.Pointer(&bubble.Info)).typ //faster than reflect 10x
	}
}

func IsNftTransfer(b *Bubble) bool {
	_, ok := b.Info.(BubbleNftTransfer)
	return ok
}
func IsJettonTransfer(b *Bubble) bool {
	_, ok := b.Info.(BubbleJettonTransfer)
	return ok
}

func IsJettonReceiver(iface abi.ContractInterface) bubbleCheck {
	return func(bubble *Bubble) bool {
		t, ok := bubble.Info.(BubbleJettonTransfer)
		if !ok || t.recipient == nil {
			return false
		}
		return t.recipient.Is(iface)
	}
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

func HasEmptyBody(b *Bubble) bool {
	return b.Info.(BubbleTx).decodedBody == nil && b.Info.(BubbleTx).opCode == nil
}

func HasOperation(name abi.MsgOpName) bubbleCheck {
	return func(b *Bubble) bool {
		return b.Info.(BubbleTx).operation(name)
	}
}
func IsAccount(id tongo.AccountID) bubbleCheck {
	return func(bubble *Bubble) bool {
		return bubble.Info.(BubbleTx).account.Address == id
	}
}
func HasTextComment(comment string) bubbleCheck {
	return func(bubble *Bubble) bool {
		body := bubble.Info.(BubbleTx).decodedBody
		if body == nil {
			return false
		}
		if body.Operation != abi.TextCommentMsgOp {
			return false
		}
		return string(body.Value.(abi.TextCommentMsgBody).Text) == comment
	}
}
func HasInterface(iface abi.ContractInterface) bubbleCheck {
	return func(bubble *Bubble) bool {
		return bubble.Info.(BubbleTx).account.Is(iface)
	}
}

func AmountInterval(min, max int64) bubbleCheck {
	return func(bubble *Bubble) bool {
		amount := bubble.Info.(BubbleTx).inputAmount
		return amount >= min && amount <= max
	}
}

func IsBounced(bubble *Bubble) bool {
	tx, ok := bubble.Info.(BubbleTx)
	return ok && tx.bounced
}

func JettonTransferOpCode(opCode uint32) bubbleCheck {
	return func(bubble *Bubble) bool {
		tx, _ := bubble.Info.(BubbleJettonTransfer)
		return tx.payload.OpCode != nil && *tx.payload.OpCode == opCode
	}
}

func JettonTransferOperation(op abi.JettonOpName) bubbleCheck {
	return func(bubble *Bubble) bool {
		tx, _ := bubble.Info.(BubbleJettonTransfer)
		return tx.payload.SumType == op
	}
}
