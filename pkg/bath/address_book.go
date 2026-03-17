package bath

import (
	"github.com/tonkeeper/tongo/ton"
)

// AddressBook provides information about well-known addresses
type AddressBook interface {
	GetGasRelayers() map[ton.AccountID]bool
}
