package i18n

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/shopspring/decimal"
)

// FormatTONs represents the given amount of nanoTONs in TONs and formats it according to the scheme (# ### or #.##)
func FormatTONs(amount int64) string {
	return FormatTokens(*big.NewInt(amount), 9, "TON")
}

// FormatTokens translates the value in indivisible units into a user-friendly form taking into account
// decimals according to the scheme (# ### or #.##)
func FormatTokens(amount big.Int, decimals int32, symbol string) string {
	x := decimal.NewFromBigInt(&amount, -1*decimals)
	x = truncate(x, 3)
	intPart := x.IntPart()
	if x.Equal(decimal.New(x.IntPart(), 0)) {
		return fmt.Sprintf("%s %s", formatIntPart(intPart), symbol)
	}
	parts := strings.Split(x.String(), ".")
	if len(parts) != 2 {
		return fmt.Sprintf("%s %s", formatIntPart(intPart), symbol)
	}
	return fmt.Sprintf("%s.%s %s", formatIntPart(intPart), parts[1], symbol)
}

func truncate(d decimal.Decimal, n int32) decimal.Decimal {
	if n <= 0 {
		return d.Truncate(n)
	}
	if d.IsZero() {
		return decimal.Zero
	}
	dn := decimal.New(1, n-1)
	if d.Abs().GreaterThanOrEqual(dn) {
		return d.Truncate(0)
	}
	for i := int32(0); i < 32; i++ {
		if d.Abs().Shift(i).GreaterThanOrEqual(dn) {
			return d.Truncate(i)
		}
	}
	return d
}

func formatIntPart(n int64) string {
	s := fmt.Sprintf("%d", n)
	length := len(s)
	if length <= 3 {
		return s
	}
	var result []string
	for length > 3 {
		result = append([]string{s[length-3:]}, result...)
		length -= 3
	}
	result = append([]string{s[:length]}, result...)
	return strings.Join(result, " ")
}
