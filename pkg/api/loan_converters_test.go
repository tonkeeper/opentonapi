package api

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
)

func TestConvertPoolRoundTransitionActionValue(t *testing.T) {
	h := &Handler{
		addressBook: mockAddressBook{
			OnGetAddressInfoByAddress: func(a tongo.AccountID) (addressbook.KnownAddress, bool) {
				return addressbook.KnownAddress{}, false
			},
		},
	}
	tests := []struct {
		name      string
		staked    int64
		recovered int64
		want      string
	}{
		{name: "gain", staked: 1_000_000_000_000, recovered: 1_234_567_890_123, want: "234 Gram"},
		{name: "loss", staked: 1_234_567_890_123, recovered: 0, want: "-1 234 Gram"},
		{name: "fractional loss", staked: 1_000_000_000, recovered: 500_000_000, want: "-0.5 Gram"},
		{name: "even", staked: 1_000_000_000, recovered: 1_000_000_000, want: "0 Gram"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &bath.PoolRoundTransitionAction{
				Pool:           ton.MustParseAccountID("0:3996a30dd8d4728d84d8596730bff7d47e2c8ef88de888bf2f6c5aef8df134dc"),
				Transition:     bath.PoolRoundSettled,
				TotalStaked:    &tt.staked,
				TotalRecovered: &tt.recovered,
			}
			_, preview := h.convertPoolRoundTransitionAction(a, "en", nil)
			require.Equal(t, tt.want, preview.Value.Value)
		})
	}
}

func TestConvertLoanRequestActionDescription(t *testing.T) {
	h := &Handler{
		addressBook: mockAddressBook{
			OnGetAddressInfoByAddress: func(a tongo.AccountID) (addressbook.KnownAddress, bool) {
				return addressbook.KnownAddress{}, false
			},
		},
	}
	amount := int64(1_000_000_000_000)
	tests := []struct {
		name       string
		amount     *int64
		roundSince uint32
		lang       string
		want       string
	}{
		{name: "amount and round", amount: &amount, roundSince: 1_700_000_000, lang: "en", want: "Requesting a loan of 1 000 Gram for the validation round starting 2023-11-14 22:13 UTC"},
		{name: "round only", roundSince: 1_700_000_000, lang: "en", want: "Requesting a loan for the validation round starting 2023-11-14 22:13 UTC"},
		{name: "amount only", amount: &amount, lang: "en", want: "Requesting a loan of 1 000 Gram"},
		{name: "neither", lang: "en", want: "Requesting a loan"},
		{name: "amount only ru", amount: &amount, lang: "ru", want: "Запрос займа 1 000 Gram"},
		{name: "neither ru", lang: "ru", want: "Запрос займа"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &bath.LoanRequestAction{
				Borrower:   ton.MustParseAccountID("0:3996a30dd8d4728d84d8596730bff7d47e2c8ef88de888bf2f6c5aef8df134dc"),
				Pool:       ton.MustParseAccountID("0:3996a30dd8d4728d84d8596730bff7d47e2c8ef88de888bf2f6c5aef8df134dc"),
				RoundSince: tt.roundSince,
				Amount:     tt.amount,
			}
			_, preview := h.convertLoanRequestAction(a, tt.lang, nil)
			require.Equal(t, tt.want, preview.Description)
		})
	}
}
