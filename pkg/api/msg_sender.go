package api

import "context"

// messageSender provides a method to send a raw message to the blockchain.
type messageSender interface {
	// SendMessage sends the given payload(a message) to the blockchain.
	SendMessage(ctx context.Context, payload []byte) error
}
