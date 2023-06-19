package i18n

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// FormatTONs represents the given amount of nanoTONs in TONs and formats it according to the english locale (#,###.##).
func FormatTONs(amount int64) string {
	p := message.NewPrinter(language.English)
	x := decimal.New(amount, -9)
	intPart := p.Sprintf("%v", x.IntPart())
	if x.Equal(decimal.New(x.IntPart(), 0)) {
		return fmt.Sprintf("%v TON", intPart)
	}
	parts := strings.Split(x.String(), ".")
	if len(parts) != 2 {
		return fmt.Sprintf("%v TON", intPart)
	}
	return fmt.Sprintf("%v.%v TON", intPart, parts[1])
}
