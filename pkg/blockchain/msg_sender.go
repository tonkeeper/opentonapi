package blockchain

import (
	"context"

	"go.uber.org/zap"

	"github.com/tonkeeper/tongo/liteapi"
)

type MsgSender struct {
	client *liteapi.Client
}

func NewMsgSender(logger *zap.Logger) (*MsgSender, error) {
	cli, err := liteapi.NewClientWithDefaultMainnet()
	if err != nil {
		return nil, err
	}
	return &MsgSender{client: cli}, nil
}

// SendMessage sends the given payload(a message) to the blockchain.
func (ms *MsgSender) SendMessage(ctx context.Context, payload []byte) error {
	_, err := ms.client.SendMessage(ctx, payload)
	return err
}
