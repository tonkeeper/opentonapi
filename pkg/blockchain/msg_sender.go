package blockchain

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/tonkeeper/tongo/config"
	"github.com/tonkeeper/tongo/liteapi"
)

// MsgSender provides a method to send a message to the blockchain.
type MsgSender struct {
	mu     sync.RWMutex
	client *liteapi.Client
	// channels is used to send a copy of payload before sending it to the blockchain.
	channels []chan []byte
	// messages is used as a cache for boc multi-sending
	messages map[string]int64 // base64, created unix time
}

func NewMsgSender(servers []config.LiteServer, channels []chan []byte) (*MsgSender, error) {
	var (
		client *liteapi.Client
		err    error
	)
	if len(servers) == 0 {
		fmt.Println("USING PUBLIC CONFIG for NewMsgSender! BE CAREFUL!")
		client, err = liteapi.NewClientWithDefaultMainnet()
	} else {
		client, err = liteapi.NewClient(liteapi.WithLiteServers(servers))
	}
	if err != nil {
		return nil, err
	}
	msgSender := &MsgSender{
		client:   client,
		channels: channels,
		messages: map[string]int64{},
	}
	go func() {
		for {
			msgSender.sendMsgsFromMempool()
			time.Sleep(time.Second * 5)
		}
	}()
	return msgSender, nil
}

func (ms *MsgSender) sendMsgsFromMempool() {
	now := time.Now().Unix()

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	for boc, createdTime := range ms.messages {
		payload, err := base64.StdEncoding.DecodeString(boc)
		if err != nil || now-createdTime > 5*60 { // ttl is 5 min
			delete(ms.messages, boc)
			continue
		}
		if err := ms.SendMessage(context.Background(), payload); err != nil {
			continue
		}
	}
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

func (ms *MsgSender) MsgsBocAddToMempool(bocMsgs []string) {
	now := time.Now().Unix()
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	for _, boc := range bocMsgs {
		ms.messages[boc] = now
	}
}
