package api

import (
	"context"
	"net/url"
	"strings"
)

type clientCtxKeyType struct{}

var clientCtxKey = clientCtxKeyType{}

// client describes who sent a request, so handlers can adapt a response to a particular
// client's quirks.
type client struct {
	userAgent string
	origin    string
}

// withClient puts the request's User-Agent and Origin into ctx.
func withClient(ctx context.Context, c client) context.Context {
	return context.WithValue(ctx, clientCtxKey, c)
}

func clientFromContext(ctx context.Context) client {
	c, _ := ctx.Value(clientCtxKey).(client)
	return c
}

// isTonkeeperUserAgent reports whether the request comes from a Tonkeeper client. Native
// clients send either the bare name or a product token with a version in the User-Agent, e.g.
// "Tonkeeper/5.0.0 (iOS 18.0)", so only the first product token is matched. The desktop client
// keeps Electron's default User-Agent, so any User-Agent mentioning Electron counts. Web clients
// can't set the User-Agent, so they are recognized by an Origin on tonkeeper.com or any of
// its subdomains, e.g. "https://wallet.tonkeeper.com".
func isTonkeeperUserAgent(c client) bool {
	return isTonkeeperOrigin(c.origin) || isTonkeeperUA(c.userAgent)
}

func isTonkeeperUA(userAgent string) bool {
	if strings.Contains(userAgent, "Electron") {
		return true
	}
	name, _, _ := strings.Cut(userAgent, "/")
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "keeper", "tonkeeper":
		return true
	default:
		return false
	}
}

func isTonkeeperOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "tonkeeper.com" || strings.HasSuffix(host, ".tonkeeper.com")
}
