package core

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/tongo"
)

type InvoiceMetadataType string

const (
	NoneInvoiceMetadataType            InvoiceMetadataType = "none"
	TextInvoiceMetadataType            InvoiceMetadataType = "text"
	EncryptedBinaryInvoiceMetadataType InvoiceMetadataType = "encrypted_binary"
)

type InvoicePayment struct {
	Source       tongo.AccountID
	Destination  tongo.AccountID
	TraceID      TraceID
	InMsgLt      uint64
	Utime        int64
	InvoiceID    uuid.UUID
	Amount       decimal.Decimal
	Currency     string
	MetadataType InvoiceMetadataType
	Metadata     []byte
}
