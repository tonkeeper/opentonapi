package core

type TrustType string

const (
	TrustWhitelist TrustType = "whitelist"
	TrustGraylist  TrustType = "graylist"
	TrustBlacklist TrustType = "blacklist"
	TrustNone      TrustType = "none"
)
