package bath

import (
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/ton"
	"golang.org/x/exp/slices"
)

type GasRelayBubble struct {
	Amount  int64
	Relayer ton.AccountID
	Target  ton.AccountID
}

func (b GasRelayBubble) ToAction() *Action {
	return &Action{
		Type: GasRelay,
		GasRelay: &GasRelayAction{
			Amount:  b.Amount,
			Relayer: b.Relayer,
			Target:  b.Target,
		},
		Success: true,
	}
}

type GasRelayAction struct {
	Amount  int64
	Relayer ton.AccountID
	Target  ton.AccountID
}

func (a GasRelayAction) SubjectAccounts() []ton.AccountID {
	return []ton.AccountID{a.Relayer} //target is not a subject account because we don't want to show it in the list of actions in wallet
}

var knownRelayers = []ton.AccountID{ //https://github.com/tonkeeper/ton-assets/blob/963996e35fe527d3d5699752eb1dc3609d5db254/accounts/infrastructure.yaml#L23C1-L54C27
	ton.MustParseAccountID("0:9b8ab637507230b99de26a55ea6d9cd4fef0cffcaafe2d1f15e835d5f5d38a43"),
	ton.MustParseAccountID("0:1c06b78eb4c0c014b51308221f6263643746fe7be60b4831a8409051cba0306f"),
	ton.MustParseAccountID("0:0bc884e676ba3dcaabe75cea71c38d6691ed0d6a89cfd95d2772c32f7be01262"),
	ton.MustParseAccountID("0:f33f5a1e309236c21fd412b9d522e24a6a6ef3745c01f7ec7d731bc0f844c334"),
	ton.MustParseAccountID("0:e3b375a5f71ea17bec125d3a88f6483575ee909b16010eefed9b64fe9b0d64e5"),
	ton.MustParseAccountID("0:73727a419e7d7f1ae1c455e58ee432f26e3a75b31078f99cedcac403f47619be"),
	ton.MustParseAccountID("0:d5a60826d1d4f157085d2bc751d037c61f1fe2d55322cd5bc0297456c513dd69"),
	ton.MustParseAccountID("0:a1809e9a6f64adde7f0f485742968433a621a4f3b5e1c5920a7077d7b63c3411"),
	ton.MustParseAccountID("0:c6f5916443f6f707139b108edce317ea52a8c4c5e5afaf9a3c6e93d64685d95d"),
	ton.MustParseAccountID("0:695a70302a23c2b4a1b987a85efa97ecbb9d1595e6f3fe95259290e04b54cf1f"),
	ton.MustParseAccountID("0:3bd3aad670168c4b1c6ec9bd29fdd65b139670b71d39c653fb5882430e1db58e"),
	ton.MustParseAccountID("0:2fa8b698bd32c65e8c936341804a7acdea1f92e3d366be4b0e243debc3dc1260"),
	ton.MustParseAccountID("0:463b36b05b642dcd8e7892796b4bf3e5aa67f1c73eec76d4be6fc2e0bcddb391"),
	ton.MustParseAccountID("0:b6eacb041642f30b424d2f6f58795e6e932acefffa779326f1c256347820b4cd"),
	ton.MustParseAccountID("0:87116a10bfc9cc340b837eb03b059ee162aaef9b6fa22dc9e435b3e73a173df9"),
	ton.MustParseAccountID("0:51b3631fe3f915981f4114cdeb1237e850b7fae43055b13dbcdc96e50f1c16ce"),
}

var GasRelayerStraw = Straw[GasRelayBubble]{
	CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.WalletSignedInternalV5R1MsgOp), HasInterface(abi.WalletV5R1), func(bubble *Bubble) bool {
		tx := bubble.Info.(BubbleTx)
		return tx.inputFrom != nil && slices.Contains(knownRelayers, tx.inputFrom.Address)
	}},
	Builder: func(newAction *GasRelayBubble, bubble *Bubble) error {
		tx := bubble.Info.(BubbleTx)
		newAction.Relayer = tx.inputFrom.Address
		newAction.Target = tx.account.Address
		newAction.Amount = tx.inputAmount
		return nil
	},
}
