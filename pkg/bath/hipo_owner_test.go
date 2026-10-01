package bath

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
)

// The owner's own view of a Hipo flow - the wallet's signing preview and the account's history,
// both FindActions with ForAccount - keeps only actions whose bubble lists the owner. Most of an
// unstake runs on Hipo's contracts, so these tests build each flow the way mainnet traces have
// it and check that the owner is shown everything done in their name.

var (
	hipoTestOwner      = tongo.MustParseAccountID("0:5718965301000000000000000000000000000000000000000000000000000001")
	hipoTestWallet     = tongo.MustParseAccountID("0:f7dea14888000000000000000000000000000000000000000000000000000002")
	hipoTestCollection = tongo.MustParseAccountID("0:83e69b2d8c000000000000000000000000000000000000000000000000000003")
	hipoTestBill       = tongo.MustParseAccountID("0:0069619e98000000000000000000000000000000000000000000000000000004")
	hipoTestProtocol   = tongo.MustParseAccountID("0:1234567890000000000000000000000000000000000000000000000000000005")
)

const (
	hipoTestTokens = 558_445_534
	hipoTestCoins  = 653_468_919
)

func hipoCoins(n int64) tlb.VarUInteger16 {
	return tlb.VarUInteger16(*big.NewInt(n))
}

// hipoMsg is one transaction of a Hipo flow: account handling a message from `from`.
func hipoMsg(account, from tongo.AccountID, op string, body any, children ...*core.Trace) *core.Trace {
	src := from
	dst := account
	trace := &core.Trace{
		Transaction: core.Transaction{
			TransactionID: core.TransactionID{Account: account},
			Type:          core.OrdinaryTx,
			Success:       true,
			InMsg: &core.Message{
				MessageID:   core.MessageID{Source: &src, Destination: &dst},
				MsgType:     core.IntMsg,
				Value:       100_000_000,
				DecodedBody: &core.DecodedMessageBody{Operation: op, Value: body},
			},
		},
		Children: children,
	}
	if account == hipoTestWallet {
		trace.AccountInterfaces = []abi.ContractInterface{abi.JettonWallet}
	}
	return trace
}

// hipoSigned is the owner's wallet sending the first message of a flow.
func hipoSigned(children ...*core.Trace) *core.Trace {
	dst := hipoTestOwner
	return &core.Trace{
		Transaction: core.Transaction{
			TransactionID: core.TransactionID{Account: hipoTestOwner},
			Type:          core.OrdinaryTx,
			Success:       true,
			InMsg:         &core.Message{MessageID: core.MessageID{Destination: &dst}, MsgType: core.ExtInMsg},
		},
		Children: children,
	}
}

func hipoReserve(mode uint8, ownershipAssigned int64, treasuryAnswer ...*core.Trace) *core.Trace {
	return hipoMsg(references.HipoParent, hipoTestWallet, abi.HipoFinanceProxyReserveTokensMsgOp,
		abi.HipoFinanceProxyReserveTokensMsgBody{Tokens: hipoCoins(hipoTestTokens), Owner: hipoTestOwner.ToMsgAddress(),
			Mode: tlb.Uint4(mode), OwnershipAssignedAmount: hipoCoins(ownershipAssigned)},
		hipoMsg(references.HipoTreasury, references.HipoParent, abi.HipoFinanceReserveTokensMsgOp,
			abi.HipoFinanceReserveTokensMsgBody{Tokens: hipoCoins(hipoTestTokens), Owner: hipoTestOwner.ToMsgAddress()},
			treasuryAnswer...),
	)
}

// hipoPayout is the treasury paying an unstake out, as it does both instantly and at round end.
func hipoPayout() *core.Trace {
	return hipoMsg(references.HipoParent, references.HipoTreasury, abi.HipoFinanceProxyTokensBurnedMsgOp,
		abi.HipoFinanceProxyTokensBurnedMsgBody{Tokens: hipoCoins(hipoTestTokens), Coins: hipoCoins(hipoTestCoins),
			Owner: hipoTestOwner.ToMsgAddress()},
		hipoMsg(hipoTestWallet, references.HipoParent, abi.HipoFinanceTokensBurnedMsgOp,
			abi.HipoFinanceTokensBurnedMsgBody{Tokens: hipoCoins(hipoTestTokens), Coins: hipoCoins(hipoTestCoins)},
			hipoMsg(hipoTestOwner, hipoTestWallet, abi.HipoFinanceWithdrawalNotificationMsgOp,
				abi.HipoFinanceWithdrawalNotificationMsgBody{Tokens: hipoCoins(hipoTestTokens), Coins: hipoCoins(hipoTestCoins)}),
		),
	)
}

// hipoBill is a bill minted for the owner; a positive ownership_assigned_amount notifies them.
func hipoBill(from tongo.AccountID, ownershipAssigned int64) *core.Trace {
	var notify []*core.Trace
	if ownershipAssigned > 0 {
		notify = append(notify, hipoMsg(hipoTestOwner, hipoTestBill, abi.NftOwnershipAssignedMsgOp, nil))
	}
	return hipoMsg(hipoTestCollection, from, abi.HipoFinanceMintBillMsgOp,
		abi.HipoFinanceMintBillMsgBody{Amount: hipoCoins(hipoTestTokens), Owner: hipoTestOwner.ToMsgAddress()},
		hipoMsg(hipoTestBill, hipoTestCollection, abi.HipoFinanceAssignBillMsgOp,
			abi.HipoFinanceAssignBillMsgBody{Amount: hipoCoins(hipoTestTokens), Owner: hipoTestOwner.ToMsgAddress()},
			notify...),
	)
}

// hipoBurn is the owner burning hGRAM on their wallet, which the jetton master turns into an unstake.
func hipoBurn(reserve *core.Trace) *core.Trace {
	return hipoMsg(hipoTestWallet, hipoTestOwner, abi.JettonBurnMsgOp,
		abi.JettonBurnMsgBody{Amount: hipoCoins(hipoTestTokens)}, reserve)
}

// hipoUnstakeAll is the comment "w": the treasury asks the jetton master to have the wallet
// unstake everything, and the wallet burns to itself.
func hipoUnstakeAll(treasuryAnswer ...*core.Trace) *core.Trace {
	return hipoSigned(
		hipoMsg(references.HipoTreasury, hipoTestOwner, abi.TextCommentMsgOp, abi.TextCommentMsgBody{Text: tlb.Text("w")},
			hipoMsg(references.HipoParent, references.HipoTreasury, "", nil,
				hipoMsg(hipoTestWallet, references.HipoParent, "", nil,
					hipoMsg(hipoTestWallet, hipoTestWallet, abi.JettonBurnMsgOp,
						abi.JettonBurnMsgBody{Amount: hipoCoins(hipoTestTokens)},
						hipoReserve(0, 0, treasuryAnswer...)),
				),
			),
		),
	)
}

func hipoStake(sender tongo.AccountID, owner tlb.MsgAddress, notify bool) *core.Trace {
	var notification []*core.Trace
	if notify {
		notification = append(notification, hipoMsg(hipoTestOwner, hipoTestWallet, abi.JettonNotifyMsgOp, nil))
	}
	return hipoMsg(references.HipoTreasury, sender, abi.HipoFinanceDepositCoinsMsgOp,
		abi.HipoFinanceDepositCoinsMsgBody{Owner: owner},
		hipoMsg(references.HipoParent, references.HipoTreasury, abi.HipoFinanceProxyTokensMintedMsgOp,
			abi.HipoFinanceProxyTokensMintedMsgBody{Tokens: hipoCoins(hipoTestTokens), Coins: hipoCoins(hipoTestCoins),
				Owner: hipoTestOwner.ToMsgAddress()},
			hipoMsg(hipoTestWallet, references.HipoParent, abi.HipoFinanceTokensMintedMsgOp,
				abi.HipoFinanceTokensMintedMsgBody{Tokens: hipoCoins(hipoTestTokens), Coins: hipoCoins(hipoTestCoins),
					Owner: hipoTestOwner.ToMsgAddress()},
				notification...),
		),
	)
}

// hipoOwnerActions runs a trace through bath twice, unfiltered and as the owner sees it, and
// returns what the owner is shown along with the owner's hGRAM balance change.
func hipoOwnerActions(t *testing.T, trace *core.Trace) (all, mine []string, hgram int64) {
	t.Helper()
	source := &mockInfoSource{
		OnJettonMastersForWallets: func(ctx context.Context, wallets []tongo.AccountID) (map[tongo.AccountID]tongo.AccountID, error) {
			return map[tongo.AccountID]tongo.AccountID{hipoTestWallet: references.HipoParent}, nil
		},
	}
	names := func(list *ActionsList) []string {
		var out []string
		for _, a := range list.Actions {
			if owner, key := hipoActionOwner(a); key != "" && owner == hipoTestOwner {
				out = append(out, key)
			}
		}
		return out
	}
	unfiltered, err := FindActions(context.Background(), trace, WithInformationSource(source))
	require.NoError(t, err)
	filtered, err := FindActions(context.Background(), trace, WithInformationSource(source), ForAccount(hipoTestOwner))
	require.NoError(t, err)
	if flow, ok := unfiltered.ValueFlow.Accounts[hipoTestOwner]; ok {
		if v, ok := flow.Jettons[references.HipoParent]; ok {
			hgram = v.Int64()
		}
	}
	return names(unfiltered), names(filtered), hgram
}

// hipoActionOwner names a Hipo action by its kind and the account it is about.
func hipoActionOwner(a Action) (tongo.AccountID, string) {
	switch {
	case a.DepositStake != nil && a.DepositStake.Implementation == core.StakingImplementationHipo:
		return a.DepositStake.Staker, "DepositStake"
	case a.WithdrawStake != nil && a.WithdrawStake.Implementation == core.StakingImplementationHipo:
		return a.WithdrawStake.Staker, "WithdrawStake"
	case a.WithdrawStakeRequest != nil && a.WithdrawStakeRequest.Implementation == core.StakingImplementationHipo:
		return a.WithdrawStakeRequest.Staker, "WithdrawStakeRequest"
	case a.JettonMint != nil && a.JettonMint.Jetton == references.HipoParent:
		return a.JettonMint.Recipient, "JettonMint"
	case a.JettonBurn != nil && a.JettonBurn.Jetton == references.HipoParent:
		return a.JettonBurn.Sender, "JettonBurn"
	}
	return tongo.AccountID{}, ""
}

func TestHipoOwnerSeesTheirOwnFlow(t *testing.T) {
	collection := hipoTestCollection
	tests := []struct {
		name  string
		trace *core.Trace
		want  []string
		// hgram is the owner's hGRAM balance change the trace should report.
		hgram int64
	}{
		{
			name:  "instant stake",
			trace: hipoSigned(hipoStake(hipoTestOwner, tlb.MsgAddress{SumType: "AddrNone"}, true)),
			want:  []string{"JettonMint", "DepositStake"},
			hgram: hipoTestTokens,
		},
		{
			// A protocol staking on the owner's behalf, with no forward fee for the notification:
			// nothing in the trace runs on the owner's account.
			name:  "stake made for the owner by someone else, without a notification",
			trace: hipoStake(hipoTestProtocol, hipoTestOwner.ToMsgAddress(), false),
			want:  []string{"JettonMint", "DepositStake"},
			hgram: hipoTestTokens,
		},
		{
			name:  "instant unstake",
			trace: hipoSigned(hipoBurn(hipoReserve(1, 0, hipoPayout()))),
			want:  []string{"JettonBurn", "WithdrawStake"},
			hgram: -hipoTestTokens,
		},
		{
			name:  "deferred unstake notifying the owner of the bill",
			trace: hipoSigned(hipoBurn(hipoReserve(2, 1, hipoBill(references.HipoTreasury, 1)))),
			want:  []string{"JettonBurn", "WithdrawStakeRequest"},
			hgram: -hipoTestTokens,
		},
		{
			// ownership_assigned_amount 0 is the simplest unstake message integrators are shown.
			name:  "deferred unstake without ownership_assigned",
			trace: hipoSigned(hipoBurn(hipoReserve(2, 0, hipoBill(references.HipoTreasury, 0)))),
			want:  []string{"JettonBurn", "WithdrawStakeRequest"},
			hgram: -hipoTestTokens,
		},
		{
			name:  "unstake-all by comment, paid now",
			trace: hipoUnstakeAll(hipoPayout()),
			want:  []string{"JettonBurn", "WithdrawStake"},
			hgram: -hipoTestTokens,
		},
		{
			name:  "unstake-all by comment, deferred",
			trace: hipoUnstakeAll(hipoBill(references.HipoTreasury, 0)),
			want:  []string{"JettonBurn", "WithdrawStakeRequest"},
			hgram: -hipoTestTokens,
		},
		{
			// The treasury cannot fund an instant unstake and hands the hGRAM back.
			name: "instant unstake rolled back",
			trace: hipoSigned(hipoBurn(hipoReserve(1, 0,
				hipoMsg(references.HipoParent, references.HipoTreasury, abi.HipoFinanceProxyRollbackUnstakeMsgOp,
					abi.HipoFinanceProxyRollbackUnstakeMsgBody{Tokens: hipoCoins(hipoTestTokens), Owner: hipoTestOwner.ToMsgAddress()},
					hipoMsg(hipoTestWallet, references.HipoParent, abi.HipoFinanceRollbackUnstakeMsgOp, nil),
				),
			))),
			want:  []string{"JettonBurn", "WithdrawStakeRequest", "JettonMint"},
			hgram: 0,
		},
		{
			name: "bill paid out at round end",
			trace: hipoMsg(references.HipoTreasury, collection, abi.HipoFinanceBurnTokensMsgOp,
				abi.HipoFinanceBurnTokensMsgBody{Tokens: hipoCoins(hipoTestTokens), Owner: hipoTestOwner.ToMsgAddress()},
				hipoPayout()),
			want: []string{"WithdrawStake"},
		},
		{
			name: "bill postponed to a later round",
			trace: hipoMsg(references.HipoTreasury, collection, abi.HipoFinanceBurnTokensMsgOp,
				abi.HipoFinanceBurnTokensMsgBody{Tokens: hipoCoins(hipoTestTokens), Owner: hipoTestOwner.ToMsgAddress()},
				hipoBill(references.HipoTreasury, 0)),
			want: []string{"WithdrawStakeRequest"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			all, mine, hgram := hipoOwnerActions(t, tt.trace)
			require.ElementsMatch(t, tt.want, all, "actions naming the owner")
			require.ElementsMatch(t, tt.want, mine, "what the owner's own view shows")
			require.Equal(t, tt.hgram, hgram, "the owner's hGRAM change")
		})
	}
}
