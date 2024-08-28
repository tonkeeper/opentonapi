package bath

import (
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/arnac-io/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo/abi"
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

	additionalInfo                  *core.TraceAdditionalInfo
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
func (b BubbleTx) ToAction() *Action {
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
			}
		}
		return nil
	}
	if b.opCode != nil && (*b.opCode != 0 && !b.operation(abi.EncryptedTextCommentMsgOp)) && b.accountWasActiveAtComputingTime && !b.account.Is(abi.Wallet) {
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
		}
	}
	a := &Action{
		TonTransfer: &TonTransferAction{
			Amount:    b.inputAmount,
			Recipient: b.account.Address,
			Sender:    b.inputFrom.Address, //can't be null because we check IsExternal
		},
		Success: true,
		Type:    TonTransfer,
	}
	if b.decodedBody != nil {
		switch s := b.decodedBody.Value.(type) {
		case abi.TextCommentMsgBody:
			converted := string(s.Text)
			a.TonTransfer.Comment = &converted
		case abi.EncryptedTextCommentMsgBody:
			a.TonTransfer.EncryptedComment = &EncryptedComment{EncryptionType: "simple", CipherText: s.CipherText}
		}
	}
	return a
}

func (b BubbleTx) operation(name string) bool {
	return b.decodedBody != nil && b.decodedBody.Operation == name
}
