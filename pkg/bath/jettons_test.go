package bath

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
)

func burnTestTx(account tongo.AccountID, iface abi.ContractInterface, from *tongo.AccountID, body *core.DecodedMessageBody, children ...*core.Trace) *core.Trace {
	trace := &core.Trace{
		Transaction: core.Transaction{
			TransactionID: core.TransactionID{Account: account},
			Success:       true,
			OrigStatus:    tlb.AccountActive,
			EndStatus:     tlb.AccountActive,
		},
		Children: children,
	}
	if iface != 0 {
		trace.AccountInterfaces = []abi.ContractInterface{iface}
	}
	if from != nil {
		trace.InMsg = &core.Message{
			MessageID:   core.MessageID{Source: from, Destination: &account},
			DecodedBody: body,
		}
	}
	return trace
}

// burnTestTrace builds sender -> wallet (jetton_burn) -> master (burn_notification).
// A nil notifiedOwner omits the burn_notification transaction.
func burnTestTrace(sender, wallet, master tongo.AccountID, senderIface abi.ContractInterface, notifiedOwner *tongo.AccountID, amount int64) *core.Trace {
	var notification []*core.Trace
	if notifiedOwner != nil {
		notification = append(notification, burnTestTx(master, abi.JettonMaster, &wallet, &core.DecodedMessageBody{
			Operation: abi.JettonBurnNotificationMsgOp,
			Value: abi.JettonBurnNotificationMsgBody{
				Amount: tlb.VarUInteger16(*big.NewInt(amount)),
				Sender: notifiedOwner.ToMsgAddress(),
			},
		}))
	}
	burn := burnTestTx(wallet, abi.JettonWallet, &sender, &core.DecodedMessageBody{
		Operation: abi.JettonBurnMsgOp,
		Value:     abi.JettonBurnMsgBody{Amount: tlb.VarUInteger16(*big.NewInt(amount))},
	}, notification...)
	return burnTestTx(sender, senderIface, nil, nil, burn)
}

func walletsInfoSource(infos map[tongo.AccountID]core.JettonWalletInfo) *mockInfoSource {
	return &mockInfoSource{
		OnJettonWalletsInfo: func(ctx context.Context, wallets []tongo.AccountID) (map[tongo.AccountID]core.JettonWalletInfo, error) {
			result := map[tongo.AccountID]core.JettonWalletInfo{}
			for _, wallet := range wallets {
				if info, ok := infos[wallet]; ok {
					result[wallet] = info
				}
			}
			return result, nil
		},
	}
}

func findJettonBurn(t *testing.T, actions []Action) *JettonBurnAction {
	t.Helper()
	for _, action := range actions {
		if action.Type == JettonBurn {
			return action.JettonBurn
		}
	}
	return nil
}

func TestJettonBurnOwner(t *testing.T) {
	owner := tongo.MustParseAccountID("0:3996a30dd8d4728d84d8596730bff7d47e2c8ef88de888bf2f6c5aef8df134dc")
	wallet := tongo.MustParseAccountID("0:fc35e51a7890d259858bd1caab05052a942006d38e95bee1d8b5045174b63653")
	master := tongo.MustParseAccountID("0:5a01bc2497790051d28ccde06c09b2a43e205d8d089370bf0490bbba093cc5af")
	const amount = 1_000_000

	tests := []struct {
		name   string
		trace  *core.Trace
		source *mockInfoSource
		// wantSender is the account that sent jetton_burn, it is also the root of the trace.
		wantSender tongo.AccountID
		// wantBurned is false when there is no burn_notification and the value flow has no jettons.
		wantBurned bool
	}{
		{
			name:       "forced burn",
			trace:      burnTestTrace(master, wallet, master, abi.JettonMaster, &owner, amount),
			source:     walletsInfoSource(map[tongo.AccountID]core.JettonWalletInfo{wallet: {Master: master, Owner: &owner}}),
			wantSender: master,
			wantBurned: true,
		},
		{
			name:       "normal burn",
			trace:      burnTestTrace(owner, wallet, master, abi.WalletV5R1, &owner, amount),
			source:     walletsInfoSource(map[tongo.AccountID]core.JettonWalletInfo{wallet: {Master: master, Owner: &owner}}),
			wantSender: owner,
			wantBurned: true,
		},
		{
			name:       "owner known only from additional info",
			trace:      burnTestTrace(master, wallet, master, abi.JettonMaster, nil, amount),
			source:     walletsInfoSource(map[tongo.AccountID]core.JettonWalletInfo{wallet: {Master: master, Owner: &owner}}),
			wantSender: master,
		},
		{
			name:       "owner known only from burn_notification",
			trace:      burnTestTrace(master, wallet, master, abi.JettonMaster, &owner, amount),
			source:     walletsInfoSource(nil),
			wantSender: master,
			wantBurned: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list, err := FindActions(context.Background(), tt.trace, WithInformationSource(tt.source))
			require.NoError(t, err)
			burn := findJettonBurn(t, list.Actions)
			require.NotNil(t, burn)
			require.Equal(t, tt.wantSender, burn.Sender)
			require.Equal(t, owner, burn.Owner)
			require.Equal(t, wallet, burn.SendersWallet)
			require.True(t, (&Action{JettonBurn: burn}).IsSubject(owner))

			if tt.wantBurned {
				require.Equal(t, master, burn.Jetton)
				ownerFlow := list.ValueFlow.Accounts[owner]
				require.NotNil(t, ownerFlow)
				burned := ownerFlow.Jettons[master]
				require.Equal(t, big.NewInt(-amount).String(), burned.String())
			} else if ownerFlow := list.ValueFlow.Accounts[owner]; ownerFlow != nil {
				require.Empty(t, ownerFlow.Jettons)
			}
			if masterFlow := list.ValueFlow.Accounts[master]; masterFlow != nil {
				require.Empty(t, masterFlow.Jettons)
			}

			ownerActions, err := FindActions(context.Background(), tt.trace, WithInformationSource(tt.source), ForAccount(owner))
			require.NoError(t, err)
			require.NotNil(t, findJettonBurn(t, ownerActions.Actions), "the owner must see the burn")
		})
	}
}

func TestJettonBurnOwnerFallsBackToSender(t *testing.T) {
	sender := tongo.MustParseAccountID("0:3996a30dd8d4728d84d8596730bff7d47e2c8ef88de888bf2f6c5aef8df134dc")
	wallet := tongo.MustParseAccountID("0:fc35e51a7890d259858bd1caab05052a942006d38e95bee1d8b5045174b63653")
	master := tongo.MustParseAccountID("0:5a01bc2497790051d28ccde06c09b2a43e205d8d089370bf0490bbba093cc5af")

	list, err := FindActions(context.Background(), burnTestTrace(sender, wallet, master, abi.WalletV5R1, nil, 1), WithInformationSource(walletsInfoSource(nil)))
	require.NoError(t, err)
	burn := findJettonBurn(t, list.Actions)
	require.NotNil(t, burn)
	require.Equal(t, sender, burn.Sender)
	require.Equal(t, sender, burn.Owner)
	require.Equal(t, []tongo.AccountID{sender}, burn.SubjectAccounts())
}
