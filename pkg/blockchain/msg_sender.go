package blockchain

import (
	"context"

	"github.com/tonkeeper/tongo/config"
	"github.com/tonkeeper/tongo/liteapi"
)

// MsgSender provides a method to send a message to the blockchain.
type MsgSender struct {
	client *liteapi.Client
	// channels is used to send a copy of payload before sending it to the blockchain.
	channels []chan []byte
}

func NewMsgSender(servers []config.LiteServer, channels []chan []byte) (*MsgSender, error) {
	var err error
	var client *liteapi.Client
	if len(servers) == 0 {
		client, err = liteapi.NewClientWithDefaultMainnet()
	} else {
		client, err = liteapi.NewClient(liteapi.WithLiteServers(servers))
	}
	if err != nil {
		return nil, err
	}
	return &MsgSender{client: client, channels: channels}, nil
}

// SendMessage sends the given payload(a message) to the blockchain.
func (ms *MsgSender) SendMessage(ctx context.Context, payload []byte) error {
	if err := liteapi.VerifySendMessagePayload(payload); err != nil {
		return err
	}
	for _, ch := range ms.channels {
		ch <- payload
	}
	_, err := ms.client.SendMessage(ctx, payload)
	return err
}
