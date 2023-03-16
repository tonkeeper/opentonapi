package core

import (
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/tongo"
)

type JettonWallet struct {
	Address       tongo.AccountID
	Balance       decimal.Decimal
	OwnerAddress  *tongo.AccountID
	JettonAddress tongo.AccountID
	Code          []byte
}

type JettonMetadata struct {
	Address      tongo.AccountID
	Verification addressbook.JettonVerificationType `json:"-"`
	Name         string                             `json:"name,omitempty"`
	Description  string                             `json:"description,omitempty"`
	Image        string                             `json:"image,omitempty"`
	Symbol       string                             `json:"symbol,omitempty"`
	Decimals     *decimal.Decimal                   `json:"decimals,omitempty"`
}
