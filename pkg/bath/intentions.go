package bath

import (
	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"golang.org/x/exp/slices"
	"reflect"
)

type OutMessage struct {
	body           any
	mode           uint8
	messageRelaxed abi.MessageRelaxed
	tx             *core.Transaction
}

func EnrichWithIntentions(trace *core.Trace, actions *ActionsList) *ActionsList {
	outMessages, inMsgCount := extractIntentions(trace)
	if len(outMessages) <= inMsgCount {
		return actions
	}
	outMessages = removeMatchedIntentions(trace, &outMessages)
	for _, outMsg := range outMessages {
		newAction := createActionFromMessage(outMsg)
		added := false
		for i, action := range actions.Actions {
			if slices.Contains(action.BaseTransactions, outMsg.tx.Hash) {
				actions.Actions = slices.Insert(actions.Actions, i+1, newAction)
				added = true
				break
			}
		}
		if !added {
			actions.Actions = append(actions.Actions, newAction)
		}
	}
	return actions
}

func extractIntentions(trace *core.Trace) ([]OutMessage, int) {
	var outMessages []OutMessage
	var inMsgCount int

	var getIntentions func(*core.Trace)
	getIntentions = func(trace *core.Trace) {
		if trace == nil {
			return
		}
		outMessages = append(outMessages, getOutMessages(&trace.Transaction)...)
		for _, child := range trace.Children {
			if child.InMsg != nil {
				inMsgCount += 1
			}
			getIntentions(child)
		}
	}
	getIntentions(trace)

	return outMessages, inMsgCount
}

func removeMatchedIntentions(trace *core.Trace, intentions *[]OutMessage) []OutMessage {
	matchedIndices := make(map[int]bool)
	var matchOutMessages func(*core.Trace)
	matchOutMessages = func(trace *core.Trace) {
		if trace == nil {
			return
		}
		for _, child := range trace.Children {
			for i, outMsg := range *intentions {
				if isMatch(outMsg, child.Transaction.InMsg) {
					matchedIndices[i] = true
				}
			}
			matchOutMessages(child)
		}
	}
	matchOutMessages(trace)

	var newIntentions []OutMessage
	for i, outMsg := range *intentions {
		if !matchedIndices[i] {
			newIntentions = append(newIntentions, outMsg)
		}
	}
	return newIntentions
}

func isMatch(msgOut OutMessage, msgIn *core.Message) bool {
	if msgIn == nil {
		return false
	}

	if !compareMessageFields(msgOut, msgIn) {
		return false
	}

	_, ok := msgOut.body.(*boc.Cell)
	if (msgOut.body == nil || ok) && msgIn.DecodedBody == nil {
		return true
	}

	if msgOut.body == nil || msgIn.DecodedBody == nil {
		return false
	}

	if reflect.TypeOf(msgOut.body) != reflect.TypeOf(msgIn.DecodedBody.Value) {
		return false
	}

	// compare message body
	switch bodyOut := msgOut.body.(type) {
	case abi.TextCommentMsgBody:
		bodyIn := msgIn.DecodedBody.Value.(abi.TextCommentMsgBody)
		return bodyOut.Text == bodyIn.Text
	case abi.JettonTransferMsgBody:
		bodyIn := msgIn.DecodedBody.Value.(abi.JettonTransferMsgBody)
		return bodyIn.QueryId == bodyOut.QueryId
	case abi.NftTransferMsgBody:
		bodyIn := msgIn.DecodedBody.Value.(abi.NftTransferMsgBody)
		return bodyIn.QueryId == bodyOut.QueryId
	case abi.DedustSwapMsgBody:
		bodyIn := msgIn.DecodedBody.Value.(abi.DedustSwapMsgBody)
		return bodyIn.QueryId == bodyOut.QueryId
	default:
		return true // not supported yet, so removed
	}
}

func compareMessageFields(msgOut OutMessage, msgIn *core.Message) bool {
	msg := msgOut.messageRelaxed.MessageInternal

	if msg.Dest != msgIn.Destination.ToMsgAddress() {
		return false
	}

	if msgOut.mode < 128 && int64(msg.Value.Grams) != msgIn.Value {
		return false
	}

	return true
}

func getOutMessages(transaction *core.Transaction) []OutMessage {
	if transaction == nil ||
		transaction.InMsg == nil ||
		transaction.InMsg.DecodedBody == nil ||
		transaction.InMsg.DecodedBody.Value == nil {
		return []OutMessage{}
	}

	var messages []OutMessage
	switch v := transaction.InMsg.DecodedBody.Value.(type) {
	case abi.WalletSignedV3ExtInMsgBody:
		for _, msg := range v.Payload {
			messages = append(messages, OutMessage{
				body:           msg.Message.MessageInternal.Body.Value.Value,
				mode:           msg.Mode,
				tx:             transaction,
				messageRelaxed: msg.Message})
		}
	case abi.WalletSignedV4ExtInMsgBody:
		for _, msg := range v.Payload {
			messages = append(messages, OutMessage{
				body:           msg.Message.MessageInternal.Body.Value.Value,
				mode:           msg.Mode,
				tx:             transaction,
				messageRelaxed: msg.Message})
		}
	case abi.WalletSignedExternalV5R1ExtInMsgBody:
		if v.Actions != nil {
			for _, msg := range *v.Actions {
				messages = append(messages, OutMessage{
					body:           msg.Msg.MessageInternal.Body.Value.Value,
					mode:           msg.Mode,
					tx:             transaction,
					messageRelaxed: msg.Msg})
			}
		}
	case abi.WalletSignedInternalV5R1MsgBody:
		if v.Actions != nil {
			for _, msg := range *v.Actions {
				messages = append(messages, OutMessage{
					body:           msg.Msg.MessageInternal.Body.Value.Value,
					mode:           msg.Mode,
					tx:             transaction,
					messageRelaxed: msg.Msg})
			}
		}
	case abi.HighloadWalletSignedV3ExtInMsgBody:
		messages = []OutMessage{{
			body:           v.Msg.MessageToSend.MessageInternal.Body.Value.Value,
			mode:           v.Msg.SendMode,
			tx:             transaction,
			messageRelaxed: v.Msg.MessageToSend}}
	}
	return messages
}

func createActionFromMessage(msgOut OutMessage) Action {
	var action Action
	switch body := msgOut.body.(type) {
	case abi.TextCommentMsgBody:
		var sender tongo.AccountID
		if msgOut.tx != nil {
			sender = msgOut.tx.Account
		}
		dest := parseAccount(msgOut.messageRelaxed.MessageInternal.Dest)
		var recipient tongo.AccountID
		if dest != nil {
			recipient = dest.Address
		}
		action = Action{Type: TonTransfer, TonTransfer: &TonTransferAction{
			Recipient: recipient,
			Sender:    sender,
			Comment:   g.Pointer(string(body.Text))}}
		if msgOut.mode < 128 && msgOut.tx != nil {
			action.TonTransfer.Amount = int64(msgOut.messageRelaxed.MessageInternal.Value.Grams)
			if msgOut.tx.EndBalance < action.TonTransfer.Amount {
				action.Error = g.Pointer("Not enough balance")
			}
		}
	case abi.NftTransferMsgBody:
		bodyNewOwner := parseAccount(body.NewOwner)
		var recipient *tongo.AccountID
		if bodyNewOwner != nil {
			recipient = &bodyNewOwner.Address
		}
		var sender *tongo.AccountID
		if msgOut.tx != nil {
			sender = &msgOut.tx.Account
		}
		dest := parseAccount(msgOut.messageRelaxed.MessageInternal.Dest)
		var nft tongo.AccountID
		if dest != nil {
			nft = dest.Address
		}
		action = Action{Type: NftItemTransfer, NftItemTransfer: &NftTransferAction{
			Recipient: recipient,
			Sender:    sender,
			Nft:       nft,
		}}
	case abi.JettonTransferMsgBody:
		bodyDest := parseAccount(body.Destination)
		var recipient *tongo.AccountID
		if bodyDest != nil {
			recipient = &bodyDest.Address
		}
		dest := parseAccount(msgOut.messageRelaxed.MessageInternal.Dest)
		var sendersWallet tongo.AccountID
		if dest != nil {
			sendersWallet = dest.Address
		}
		var sender *tongo.AccountID
		if msgOut.tx != nil {
			sender = &msgOut.tx.Account
		}
		action = Action{Type: JettonTransfer, JettonTransfer: &JettonTransferAction{
			Recipient:     recipient,
			Sender:        sender,
			Amount:        body.Amount,
			SendersWallet: sendersWallet,
		}}
	default:
		dest := parseAccount(msgOut.messageRelaxed.MessageInternal.Dest)
		var recipient tongo.AccountID
		if dest != nil {
			recipient = dest.Address
		}
		var sender tongo.AccountID
		if msgOut.tx != nil {
			sender = msgOut.tx.Account
		}
		action = Action{Type: TonTransfer, TonTransfer: &TonTransferAction{
			Recipient: recipient,
			Sender:    sender}}
		if msgOut.mode < 128 && msgOut.tx != nil {
			action.TonTransfer.Amount = int64(msgOut.messageRelaxed.MessageInternal.Value.Grams)
			if msgOut.tx.EndBalance < action.TonTransfer.Amount {
				action.Error = g.Pointer("Not enough balance")
			}
		}
	}

	action.Success = false
	return action
}
