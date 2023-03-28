package api

import (
	"fmt"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"strconv"
	"strings"
)

func convertToApiJetton(metadata tongo.JettonMetadata, master tongo.AccountID, imgGenerator previewGenerator) (oas.Jetton, error) {
	convertVerification, _ := convertJettonVerification(addressbook.None) // TODO: change to real verify
	name := metadata.Name
	if name == "" {
		name = "Unknown Token"
	}
	symbol := metadata.Symbol
	if symbol == "" {
		symbol = "UKWN"
	}
	normalizedSymbol := strings.TrimSpace(strings.ToUpper(symbol))
	if normalizedSymbol == "TON" || normalizedSymbol == "TОN" { //eng and russian
		symbol = "SCAM"
	}
	jetton := oas.Jetton{
		Address:      master.ToRaw(),
		Name:         name,
		Symbol:       symbol,
		Verification: oas.OptJettonVerificationType{Value: convertVerification},
	}
	dec, err := convertJettonDecimals(metadata.Decimals)
	if err != nil {
		return oas.Jetton{}, err
	}
	jetton.Decimals = dec
	if metadata.Image != "" {
		preview := imgGenerator.GenerateImageUrl(metadata.Image, 200, 200)
		jetton.Image = oas.OptString{Value: preview}
	}
	return jetton, nil
}

func convertJettonDecimals(decimals string) (int, error) {
	if decimals == "" {
		return 9, nil
	}
	dec, err := strconv.Atoi(decimals)
	if err != nil {
		return 0, err
	}
	return dec, nil
}

func convertJettonVerification(verificationType addressbook.JettonVerificationType) (oas.JettonVerificationType, error) {
	switch verificationType {
	case addressbook.Whitelist:
		return oas.JettonVerificationTypeWhitelist, nil
	case addressbook.Blacklist:
		return oas.JettonVerificationTypeBlacklist, nil
	case addressbook.None:
		return oas.JettonVerificationTypeNone, nil
	default:
		// if we do not find matches, then we throw out an error and set a default api.JettonVerificationTypeNone
		return oas.JettonVerificationTypeNone, fmt.Errorf("convert jetton verification error: %v", verificationType)
	}
}
