package core

import "github.com/tonkeeper/tongo"

// StorageProvider is a smart contract that accepts storage requests and manages payments from clients.
type StorageProvider struct {
	Address            tongo.AccountID
	AcceptNewContracts bool
	//  RatePerMbDay specifies the cost of storage in nanoTON per megabyte per day.
	RatePerMbDay int64
	// MaxSpan specifies how often the provider provides proofs of Bag storage.
	MaxSpan int64
	// MinimalFileSize specifies min Bag size in bytes.
	MinimalFileSize int64
	// MaximalFileSize specifies max Bag size in bytes.
	MaximalFileSize int64
}
