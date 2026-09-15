package api

import (
	"context"
	"strings"
)

type userAgentCtxKeyType struct{}

var userAgentCtxKey = userAgentCtxKeyType{}

// withUserAgent puts the request's User-Agent into ctx so handlers can adapt a response
// to a particular client's quirks.
func withUserAgent(ctx context.Context, userAgent string) context.Context {
	return context.WithValue(ctx, userAgentCtxKey, userAgent)
}

func userAgentFromContext(ctx context.Context) string {
	userAgent, _ := ctx.Value(userAgentCtxKey).(string)
	return userAgent
}

// isTonkeeperUserAgent reports whether the User-Agent belongs to a Tonkeeper client. Clients
// send either the bare name or a product token with a version, e.g. "Tonkeeper/5.0.0 (iOS 18.0)",
// so only the first product token is matched.
func isTonkeeperUserAgent(userAgent string) bool {
	name, _, _ := strings.Cut(userAgent, "/")
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "keeper", "tonkeeper":
		return true
	default:
		return false
	}
}
