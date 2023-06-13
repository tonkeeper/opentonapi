package bath

import (
	"github.com/tonkeeper/tongo/abi"
)

type Matcher struct {
	CheckFunc func(bubble Bubble) bool
	Children  []Matcher
}

var GetGemsSale = Matcher{
	CheckFunc: func(bubble Bubble) bool {
		txBubble, ok := bubble.Info.(BubbleTx)
		if !ok {
			return false
		}
		if !txBubble.account.Is(abi.NftSaleGetgems) && !txBubble.account.Is(abi.NftSale) {
			return false
		}
		return true
	},
	Children: []Matcher{
		{
			CheckFunc: func(bubble Bubble) bool {
				_, ok := bubble.Info.(BubbleNftTransfer)
				return ok
			},
		},
	},
}

func (m Matcher) Match(bubble Bubble) {

}
