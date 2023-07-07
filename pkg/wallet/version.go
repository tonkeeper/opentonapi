package wallet

import (
	"fmt"

	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/wallet"
)

func GetVersionByCode(code []byte) (wallet.Version, error) {
	if len(code) == 0 {
		return 0, fmt.Errorf("can't work with a wallet without code")
	}
	cells, err := boc.DeserializeBoc(code)
	if err != nil {
		return 0, err
	}
	if len(cells) != 1 {
		return 0, fmt.Errorf("can't get wallet version because its code contains multiple root cells")
	}
	hash, err := cells[0].Hash()
	if err != nil {
		return 0, fmt.Errorf("failed to calculate code hash: %w", err)
	}
	walletVersion, ok := wallet.GetVerByCodeHash(tlb.Bits256(hash))
	if !ok {
		return 0, fmt.Errorf("not a wallet")
	}
	return walletVersion, nil
}
