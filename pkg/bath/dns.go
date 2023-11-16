package bath

import (
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

type BubbleDnsItemRenew struct {
	DnsRenewAction
	Success bool
}

type DnsRenewAction struct {
	Item    ton.AccountID
	Renewer ton.AccountID
}

func (b BubbleDnsItemRenew) ToAction() *Action {
	return &Action{Success: b.Success, Type: DnsRenew, DnsRenew: &b.DnsRenewAction}
}

func (a DnsRenewAction) SubjectAccounts() []ton.AccountID {
	return []ton.AccountID{a.Renewer, a.Item}
}

var DNSRenewStraw = Straw[BubbleDnsItemRenew]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.DeleteDnsRecordMsgOp), HasInterface(abi.NftItem), func(bubble *Bubble) bool {
		return bubble.Info.(BubbleTx).decodedBody.Value.(abi.DeleteDnsRecordMsgBody).Key.Equal(tlb.Bits256{})
	}},
	Builder: func(newAction *BubbleDnsItemRenew, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Renewer = tx.inputFrom.Address
		newAction.Item = tx.account.Address
		return nil
	},
	SingleChild: &Straw[BubbleDnsItemRenew]{
		Optional:   true,
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.BounceMsgOp)},
	},
}
