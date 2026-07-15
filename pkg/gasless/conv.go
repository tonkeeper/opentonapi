package gasless

import (
	"fmt"
	"strconv"

	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	tonwallet "github.com/tonkeeper/tongo/wallet"
)

// ToWalletMessage converts a battery-built internal message back into a RawMessage.
func (m Message) ToWalletMessage() (tonwallet.RawMessage, error) {
	address, err := ton.ParseAccountID(m.Address)
	if err != nil {
		return tonwallet.RawMessage{}, err
	}
	amount, err := strconv.ParseUint(m.Amount, 10, 64)
	if err != nil {
		return tonwallet.RawMessage{}, fmt.Errorf("invalid amount: %w", err)
	}
	msg := tonwallet.Message{
		Amount:  tlb.Grams(amount),
		Address: address,
		Bounce:  true,
		Mode:    tonwallet.DefaultMessageMode,
	}
	if m.Payload != "" {
		msg.Body, err = boc.DeserializeSinglRootHex(m.Payload)
		if err != nil {
			return tonwallet.RawMessage{}, fmt.Errorf("cant parse gassless message payload: %v", err)
		}
	}
	if m.StateInit != "" {
		cell, err := boc.DeserializeSinglRootHex(m.StateInit)
		if err != nil {
			return tonwallet.RawMessage{}, fmt.Errorf("invalid state init: %v", err)
		}
		var init tlb.StateInit
		if err := tlb.Unmarshal(cell, &init); err != nil {
			return tonwallet.RawMessage{}, err
		}
		msg.Init = &init
	}
	return tonwallet.ToRawMessage(msg)
}
