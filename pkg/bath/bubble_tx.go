package bath

import (
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo/abi"
)

type BubbleTx struct {
	success          bool
	transactionType  core.TransactionType
	inputAmount      int64
	inputExtraAmount core.ExtraCurrencies
	inputFrom        *Account
	bounce           bool
	bounced          bool
	external         bool
	account          Account
	opCode           *uint32
	decodedBody      *core.DecodedMessageBody
	init             []byte

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
	if b.opCode != nil && (*b.opCode != 0 && !b.operation(abi.EncryptedTextCommentMsgOp)) && b.accountWasActiveAtComputingTime && !b.account.Is(abi.Wallet) && len(b.inputExtraAmount) == 0 {
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
	var (
		comment          *string
		encryptedComment *EncryptedComment
	)
	if b.decodedBody != nil {
		switch s := b.decodedBody.Value.(type) {
		case abi.TextCommentMsgBody:
			converted := string(s.Text)
			comment = &converted
		case abi.EncryptedTextCommentMsgBody:
			encryptedComment = &EncryptedComment{EncryptionType: "simple", CipherText: s.CipherText}
		}
	}
	if len(b.inputExtraAmount) > 0 {
		action := Action{
			ExtraCurrencyTransfer: &ExtraCurrencyTransferAction{
				Recipient:        b.account.Address,
				Sender:           b.inputFrom.Address, //can't be null because we check IsExternal
				Comment:          comment,
				EncryptedComment: encryptedComment,
			},
			Success: true,
			Type:    ExtraCurrencyTransfer,
		}
		for id, amount := range b.inputExtraAmount {
			action.ExtraCurrencyTransfer.CurrencyID = id
			action.ExtraCurrencyTransfer.Amount = amount
			break // TODO: extract more than one currency
		}
		return &action
	}
	return &Action{
		TonTransfer: &TonTransferAction{
			Amount:           b.inputAmount,
			Recipient:        b.account.Address,
			Sender:           b.inputFrom.Address, //can't be null because we check IsExternal
			Comment:          comment,
			EncryptedComment: encryptedComment,
		},
		Success: true,
		Type:    TonTransfer,
	}
}

func (b BubbleTx) operation(name string) bool {
	return b.decodedBody != nil && b.decodedBody.Operation == name
}
