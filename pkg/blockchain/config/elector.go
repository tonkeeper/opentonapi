package config

import (
	"github.com/tonkeeper/tongo"
)

func ElectorAddress() tongo.AccountID {
	// TODO: read from the blockchain config
	return tongo.MustParseAccountID("Ef8zMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzM0vF")
}
