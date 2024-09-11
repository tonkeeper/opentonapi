package bath

import (
	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo/abi"
	"reflect"
)

type Intention struct {
	ExpectedAction   ActionType
	IncompleteAction Action
}

func EnrichWithIntentions(transaction core.Transaction, actions *ActionsList) *ActionsList {
	var intentions []Intention
	if transaction.InMsg != nil && transaction.InMsg.MsgType == core.ExtInMsg {
		intentions = extractIntentions(transaction)
	}
	return matchIntentionsWithActions(intentions, actions)
}

// TODO: add logic for internal messages
func extractIntentions(transaction core.Transaction) []Intention {
	var intentions []Intention
	switch v := transaction.InMsg.DecodedBody.Value.(type) {
	case abi.WalletSignedV3ExtInMsgBody:
		for _, msg := range v.Payload {
			intentions = append(intentions, extractIntentionFromMessage(msg.Message, transaction))
		}
	case abi.WalletSignedV4ExtInMsgBody:
		for _, msg := range v.Payload {
			intentions = append(intentions, extractIntentionFromMessage(msg.Message, transaction))
		}
	case abi.WalletSignedExternalV5R1ExtInMsgBody:
		if v.Actions != nil {
			for _, msg := range *v.Actions {
				intentions = append(intentions, extractIntentionFromMessage(msg.Msg, transaction))
			}
		}
	case abi.HighloadWalletSignedV3ExtInMsgBody:
		intentions = []Intention{extractIntentionFromMessage(v.Msg.MessageToSend, transaction)}
	}
	return intentions
}

func extractIntentionFromMessage(message abi.MessageRelaxed, transaction core.Transaction) Intention {
	if message.SumType == "MessageInternal" {
		if message.MessageInternal.Body.Value.Value == nil {
			return Intention{ExpectedAction: TonTransfer, IncompleteAction: Action{TonTransfer: &TonTransferAction{
				Amount:    int64(message.MessageInternal.Value.Grams),
				Recipient: parseAccount(message.MessageInternal.Dest).Address,
				Sender:    transaction.Account}}}
		}

		switch v := message.MessageInternal.Body.Value.Value.(type) {
		case abi.TextCommentMsgBody:
			return Intention{ExpectedAction: TonTransfer,
				IncompleteAction: Action{TonTransfer: &TonTransferAction{
					Amount:    int64(message.MessageInternal.Value.Grams),
					Recipient: parseAccount(message.MessageInternal.Dest).Address,
					Sender:    transaction.Account,
					Comment:   g.Pointer(string(v.Text))}}}
			//case abi.NftTransferMsgBody:
			//	return Intention{ExpectedAction: NftItemTransfer}
			//case abi.JettonTransferMsgBody:
			//	return Intention{ExpectedAction: JettonTransfer}
			//case abi.DedustSwapMsgBody:
			//	return Intention{ExpectedAction: JettonSwap, IncompleteAction: Action{JettonSwap: &JettonSwapAction{Dex: Dedust}}}
		}
	}

	return Intention{}
}

func matchIntentionsWithActions(intentions []Intention, actionsList *ActionsList) *ActionsList {
	result := actionsList.Actions
	matchedActions := make([]bool, len(actionsList.Actions))
	for _, intention := range intentions {
		matchedAction := -1
		for i, action := range actionsList.Actions {
			if !matchedActions[i] && isPartialMatch(intention.IncompleteAction, action) {
				matchedAction = i
				matchedActions[matchedAction] = true
				break
			}
		}
		if matchedAction == -1 {
			newAction := Action{
				Type:    intention.ExpectedAction,
				Success: false, // TODO: provide a reason why failed in Description
			}

			switch newAction.Type {
			case TonTransfer:
				newAction.TonTransfer = intention.IncompleteAction.TonTransfer
			}
			result = append(result, newAction)
		}
	}
	return &ActionsList{ValueFlow: actionsList.ValueFlow, Actions: result}
}

func isPartialMatch(incompleteAction, action any) bool {
	val1 := reflect.ValueOf(incompleteAction)
	val2 := reflect.ValueOf(action)

	if val1.Kind() != val2.Kind() {
		return false
	}

	// Handle pointer types
	if val1.Kind() == reflect.Ptr {
		if val1.IsNil() {
			return true
		}
		if !val1.IsNil() && val2.IsNil() {
			return false
		}
		return isPartialMatch(val1.Elem().Interface(), val2.Elem().Interface())
	}

	// Handle struct types
	if val1.Kind() == reflect.Struct {
		typ1 := val1.Type()
		typ2 := val2.Type()

		if typ1 != typ2 {
			return false
		}

		for i := 0; i < val1.NumField(); i++ {
			field1 := val1.Field(i)
			field2 := val2.Field(i)

			if !isPartialMatch(field1.Interface(), field2.Interface()) {
				return false
			}
		}
		return true
	}

	// Handle other types (primitive types, slices, etc.)
	zeroVal := reflect.Zero(val1.Type()).Interface()
	if reflect.DeepEqual(val1.Interface(), zeroVal) {
		return true
	}

	return reflect.DeepEqual(val1.Interface(), val2.Interface())
}
