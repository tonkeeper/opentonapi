package core

import (
	"github.com/google/uuid"
	"github.com/tonkeeper/tongo"
)

type PurchaseMetadataType string

type PurchaseMetadata struct {
	Type    PurchaseMetadataType
	Payload []byte
}

const (
	NoneMetadataType            PurchaseMetadataType = "none"
	TextMetadataType            PurchaseMetadataType = "text"
	EncryptedBinaryMetadataType PurchaseMetadataType = "encrypted_binary"
)

type InvoicePayment struct {
	Source      tongo.AccountID
	Destination tongo.AccountID
	TraceID     TraceID
	InMsgLt     uint64
	Utime       int64
	InvoiceID   uuid.UUID
	Amount      Price
	Metadata    PurchaseMetadata
}
