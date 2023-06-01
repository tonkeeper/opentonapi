package bath

import (
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/utils"

	"github.com/tonkeeper/opentonapi/pkg/core"
)

type BubbleTx struct {
	success         bool
	transactionType core.TransactionType
	inputAmount     int64
	inputFrom       *Account
	bounce          bool
	bounced         bool
	external        bool
	account         Account
	opCode          *uint32
	decodedBody     *core.DecodedMessageBody
	init            []byte

	additionalInfo                  map[string]interface{} //place for storing different data from trace which can be useful later
	accountWasActiveAtComputingTime bool
}

func (b BubbleTx) String() string {
	return fmt.Sprintf("success: %v bounce: %v, bounced: %v,  account: %v, body: %v", b.success, b.bounce, b.bounced, b.account, b.decodedBody)
}

func dumpCallArgs(v any) string {
	bs, err := yaml.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(bs)
}
func (b BubbleTx) ToAction(book addressBook) *Action {
	if b.external {
		if b.transactionType == core.TickTockTx {
			return &Action{
				SmartContractExec: &SmartContractAction{
					TonAttached: b.inputAmount,
					Executor:    b.account.Address,
					Contract:    b.account.Address,
					Operation:   "Tick-tock",
				},
				Success: b.success,
				Type:    SmartContractExec,
				SimplePreview: SimplePreview{
					Name:      "Smart Contract Execution",
					MessageID: smartContractMessageID,
					Accounts:  []tongo.AccountID{b.account.Address},
				},
			}
		}
		return nil
	}
	if b.opCode != nil && *b.opCode != 0 && b.accountWasActiveAtComputingTime && !b.account.Is(abi.Wallet) {
		operation := fmt.Sprintf("0x%08x", *b.opCode)
		payload := ""
		if b.decodedBody != nil {
			operation = b.decodedBody.Operation
			payload = dumpCallArgs(b.decodedBody.Value)
		}
		return &Action{
			SmartContractExec: &SmartContractAction{
				TonAttached: b.inputAmount,
				Executor:    b.inputFrom.Address, //can't be null because we check IsExternal
				Contract:    b.account.Address,
				Operation:   operation,
				Payload:     payload,
			},
			Success: b.success,
			Type:    SmartContractExec,
			SimplePreview: SimplePreview{
				Name:      "Smart Contract Execution",
				MessageID: smartContractMessageID,
				Accounts:  distinctAccounts(b.account.Address, b.inputFrom.Address),
			},
		}
	}
	value := utils.HumanFriendlyCoinsRepr(b.inputAmount)
	a := &Action{
		TonTransfer: &TonTransferAction{
			Amount:    b.inputAmount,
			Recipient: b.account.Address,
			Sender:    b.inputFrom.Address, //can't be null because we check IsExternal
		},
		Success: true,
		Type:    TonTransfer,
		SimplePreview: SimplePreview{
			Name:      "Ton Transfer",
			MessageID: tonTransferMessageID,
			TemplateData: map[string]interface{}{
				"Value": value,
			},
			Accounts: distinctAccounts(b.account.Address, b.inputFrom.Address),
			Value:    value,
		},
	}
	if b.decodedBody != nil {
		s, ok := b.decodedBody.Value.(abi.TextCommentMsgBody)
		if ok {
			converted := string(s.Text)
			a.TonTransfer.Comment = &converted
		}
	}
	return a
}

func (b BubbleTx) operation(name string) bool {
	return b.decodedBody != nil && b.decodedBody.Operation == name
}

func (b BubbleTx) tonAttached() HiddenTonValue {
	var senderAddr tongo.AccountID
	if b.inputFrom != nil {
		senderAddr = b.inputFrom.Address
	}
	return HiddenTonValue{
		Amount:   b.inputAmount,
		Sender:   senderAddr,
		Receiver: b.account.Address,
	}
}
