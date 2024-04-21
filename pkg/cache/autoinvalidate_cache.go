package cache

import (
	"fmt"
	"github.com/tonkeeper/tongo/ton"
	"strings"
)

const keySeparator rune = '#'

// IN DEVELOPING!
type AutoInvalidateByAccountCache struct {
	accounts map[ton.AccountID][]string
	blocks   map[ton.BlockID]string
	cache    map[string]map[string]any
}

// IN DEVELOPING!
func (c AutoInvalidateByAccountCache) Set(operation string, value any, keys ...any) {
	if len(keys) == 0 {
		panic("keys is empty")
	}
	var keyBuilder strings.Builder
	var blocks []ton.BlockID
	var accounts []ton.AccountID
	for _, k := range keys {
		if a, ok := k.(ton.AccountID); ok {
			accounts = append(accounts, a)
		}
		if b, ok := k.(ton.BlockID); ok {
			blocks = append(blocks, b)
		}
		keyBuilder.WriteRune(keySeparator)
		keyBuilder.WriteString(strings.Map(func(r rune) rune {
			if r == keySeparator {
				return -1
			}
			return r
		}, fmt.Sprintf("%v", k)))
	}
	key := keyBuilder.String()
	c.cache[operation][key] = value

}
