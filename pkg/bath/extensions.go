package bath

import (
	"github.com/tonkeeper/tongo"
)

type BubbleAddExtension struct {
	Wallet    tongo.AccountID
	Extension tongo.AccountID
	Success   bool
}

type BubbleRemoveExtension struct {
	Wallet    tongo.AccountID
	Extension tongo.AccountID
	Success   bool
}

type BubbleSetSignatureAllowed struct {
	Wallet           tongo.AccountID
	SignatureAllowed bool
	Success          bool
}

func (b BubbleAddExtension) ToAction() *Action {
	return &Action{
		AddExtension: &AddExtensionAction{
			Wallet:    b.Wallet,
			Extension: b.Extension,
		},
		Type:    AddExtension,
		Success: b.Success,
	}
}

func (b BubbleRemoveExtension) ToAction() *Action {
	return &Action{
		RemoveExtension: &RemoveExtensionAction{
			Wallet:    b.Wallet,
			Extension: b.Extension,
		},
		Type:    RemoveExtension,
		Success: b.Success,
	}
}

func (b BubbleSetSignatureAllowed) ToAction() *Action {
	return &Action{
		SetSignatureAllowed: &SetSignatureAllowedAction{
			Wallet:           b.Wallet,
			SignatureAllowed: b.SignatureAllowed,
		},
		Type:    SetSignatureAllowed,
		Success: b.Success,
	}
}
