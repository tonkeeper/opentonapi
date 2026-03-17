package bath

import (
	"reflect"

	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/ton"
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

var defaultGasProxyRelayers = map[ton.AccountID]bool{
	// https://github.com/tonkeeper/ton-assets/blob/7d40927aa9dab54bf4dbf58ef44b1e01995efea8/accounts/infrastructure.yaml#L23
	// currently used relayers
	ton.MustParseAccountID("0:e065d735fc349ad0d8674033798b99af76f8dce671a9c49d4fe9164e644a6a52"): true,
	ton.MustParseAccountID("0:0dc09ed193e47757057ed6eee7fb0677719f31bd8828113ed52d47abf23447da"): true,
	ton.MustParseAccountID("0:736118ab10bedfb4031dc904ec9d7c9061a905f6efa5d8f55d684233b715b57b"): true,
	ton.MustParseAccountID("0:d8a041801857f7132d1dc9b9dba617836a87cf3914aebbe3d7415ebe925f40cb"): true,
	ton.MustParseAccountID("0:147cb558f69a4d95d99f76a6d600e839172eecbf4c06d1a2695f430274fa956f"): true,
	ton.MustParseAccountID("0:3da40403e1e387c49e1370f47cc83aed6b5d16911495942788c6eff94ceec39d"): true,
	ton.MustParseAccountID("0:a8ddb23c8ffd94fa844cfe0fbce907f4ccf01e24ab0b135929079edc50f43e4b"): true,
	ton.MustParseAccountID("0:f0f44efb4627d8e5cf595f35c83b5f94f46f3e6d162bd765b540364c5fc08a29"): true,
	ton.MustParseAccountID("0:bd2c0a589c6b68eae98c90c6e9517d2357041693bd6eca2c24aa9ffac9f7a51d"): true,
	ton.MustParseAccountID("0:6b2bcd8f945e64994818a4ca3fd79caba83d8a41edc102f8ff4d93d31d47815e"): true,
	ton.MustParseAccountID("0:407a1a5774ed6dc0cb50aa5d300e951eb1ab097281b2d1890d96baf5894f3416"): true,
	ton.MustParseAccountID("0:c1d82b71f1a5cccfb520313f2d8689950b6deeb24c9a43eeb5d5fe851390008e"): true,
	ton.MustParseAccountID("0:5a3a8cb244414663d8669f0f3716b9892127dfd16a8b9daac48ea34386261bca"): true,
	ton.MustParseAccountID("0:2ae6b1f8033693865134e098f0380b9580c00169b412d2c5be85abf5001753f1"): true,
	ton.MustParseAccountID("0:93b37955923cf99faeb772929fa75d92bf00db01aa92fd66174d904852a1bf90"): true,
	ton.MustParseAccountID("0:89009a16daa89aa2f9969ba0a6c98a7a692e94731d34fcb530fc25c5857a5d7b"): true,
	// previously used relayers
	ton.MustParseAccountID("0:9b8ab637507230b99de26a55ea6d9cd4fef0cffcaafe2d1f15e835d5f5d38a43"): true,
	ton.MustParseAccountID("0:1c06b78eb4c0c014b51308221f6263643746fe7be60b4831a8409051cba0306f"): true,
	ton.MustParseAccountID("0:0bc884e676ba3dcaabe75cea71c38d6691ed0d6a89cfd95d2772c32f7be01262"): true,
	ton.MustParseAccountID("0:f33f5a1e309236c21fd412b9d522e24a6a6ef3745c01f7ec7d731bc0f844c334"): true,
	ton.MustParseAccountID("0:e3b375a5f71ea17bec125d3a88f6483575ee909b16010eefed9b64fe9b0d64e5"): true,
	ton.MustParseAccountID("0:73727a419e7d7f1ae1c455e58ee432f26e3a75b31078f99cedcac403f47619be"): true,
	ton.MustParseAccountID("0:d5a60826d1d4f157085d2bc751d037c61f1fe2d55322cd5bc0297456c513dd69"): true,
	ton.MustParseAccountID("0:a1809e9a6f64adde7f0f485742968433a621a4f3b5e1c5920a7077d7b63c3411"): true,
	ton.MustParseAccountID("0:c6f5916443f6f707139b108edce317ea52a8c4c5e5afaf9a3c6e93d64685d95d"): true,
	ton.MustParseAccountID("0:695a70302a23c2b4a1b987a85efa97ecbb9d1595e6f3fe95259290e04b54cf1f"): true,
	ton.MustParseAccountID("0:3bd3aad670168c4b1c6ec9bd29fdd65b139670b71d39c653fb5882430e1db58e"): true,
	ton.MustParseAccountID("0:2fa8b698bd32c65e8c936341804a7acdea1f92e3d366be4b0e243debc3dc1260"): true,
	ton.MustParseAccountID("0:463b36b05b642dcd8e7892796b4bf3e5aa67f1c73eec76d4be6fc2e0bcddb391"): true,
	ton.MustParseAccountID("0:b6eacb041642f30b424d2f6f58795e6e932acefffa779326f1c256347820b4cd"): true,
	ton.MustParseAccountID("0:87116a10bfc9cc340b837eb03b059ee162aaef9b6fa22dc9e435b3e73a173df9"): true,
	ton.MustParseAccountID("0:51b3631fe3f915981f4114cdeb1237e850b7fae43055b13dbcdc96e50f1c16ce"): true,
}

func GasRelayerStraw(book AddressBook) Straw[GasRelayBubble] {
	return Straw[GasRelayBubble]{
		CheckFuncs: []bubbleCheck{IsTx, HasOperation(abi.WalletSignedInternalV5R1MsgOp), HasInterface(abi.WalletV5R1), func(bubble *Bubble) bool {
			tx := bubble.Info.(BubbleTx)
			return tx.inputFrom != nil && isGasRelayer(tx.inputFrom.Address, book)
		}},
		Builder: func(newAction *GasRelayBubble, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			newAction.Relayer = tx.inputFrom.Address
			newAction.Target = tx.account.Address
			newAction.Amount = tx.inputAmount
			return nil
		},
	}
}

func gasRelayerSet(book AddressBook) map[ton.AccountID]bool {
	if book == nil || reflect.ValueOf(book).IsNil() {
		return defaultGasProxyRelayers
	}
	relayers := book.GetGasRelayers()
	if len(relayers) == 0 {
		// book is not expected to be empty, likely some problem with data format
		return defaultGasProxyRelayers
	}
	return relayers
}

func isGasRelayer(addr ton.AccountID, book AddressBook) bool {
	return gasRelayerSet(book)[addr]
}
