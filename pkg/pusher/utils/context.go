package utils

import (
	"context"
)

const TokenNameKey = "token-name-key"

// TokenNameFromContext returns a token name from a request context.
// Can be added by auth middleware.
func TokenNameFromContext(ctx context.Context) string {
	name, _ := ctx.Value(TokenNameKey).(string)
	return name
}
