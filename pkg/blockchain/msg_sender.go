package blockchain

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/config"
	"github.com/tonkeeper/tongo/liteapi"
	"go.uber.org/zap"
)

const ttl = 5 * 60 // in seconds

// MsgSender provides a method to send a message to the blockchain.
type MsgSender struct {
	logger *zap.Logger
	client *liteapi.Client
	// receivers get a copy of a message before sending it to the blockchain.
	// receivers is a read-only map/field.
	receivers map[string]chan<- ExtInMsgCopy

	// mu protects "batches".
	mu sync.Mutex
	// batches is used as a cache for boc multi-sending.
	batches []batchOfMessages
}

type batchOfMessages struct {
	Copies []ExtInMsgCopy
	RecvAt int64
}

// ExtInMsgCopy represents an external message we receive on /v2/blockchain/message endpoint.
type ExtInMsgCopy struct {
	// MsgBoc is a base64 encoded message boc.
	MsgBoc string
	// Payload is a decoded message boc.
	Payload []byte
	// Details contains some optional details from a request context.
	Details any
	// Accounts is set when the message is emulated.
	Accounts map[tongo.AccountID]struct{}
}

func (m *ExtInMsgCopy) IsEmulation() bool {
	return len(m.Accounts) > 0
}

func NewMsgSender(logger *zap.Logger, servers []config.LiteServer, receivers map[string]chan<- ExtInMsgCopy) (*MsgSender, error) {
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
		client:    client,
		logger:    logger,
		receivers: receivers,
	}
	go func() {
		for {
			msgSender.dropExpiredBatches()
			msgSender.sendBatches()
			time.Sleep(time.Second * 5)
		}
	}()
	return msgSender, nil
}

func (ms *MsgSender) dropExpiredBatches() {
	now := time.Now().Unix()
	ms.mu.Lock()
	defer ms.mu.Unlock()
	var batches []batchOfMessages
	for _, batch := range ms.batches {
		if now-batch.RecvAt > ttl {
			continue
		}
		batches = append(batches, batch)
	}
	ms.batches = batches
}

func (ms *MsgSender) batchesReadyForSending() []batchOfMessages {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.batches
}

func (ms *MsgSender) sendBatches() {
	batches := ms.batchesReadyForSending()
	for _, batch := range batches {
		for _, msgCopy := range batch.Copies {
			if err := ms.sendMessageFromBatch(msgCopy); err != nil {
				// TODO: remove from the queue on success? log error?
				continue
			}
		}
	}
}

// SendMessage sends the given a message to the blockchain.
func (ms *MsgSender) SendMessage(ctx context.Context, msgCopy ExtInMsgCopy) error {
	if err := liteapi.VerifySendMessagePayload(msgCopy.Payload); err != nil {
		return err
	}
	for name, ch := range ms.receivers {
		select {
		case ch <- msgCopy:
		default:
			ms.logger.Warn("receiver is too slow", zap.String("name", name))
		}

	}
	_, err := ms.client.SendMessage(ctx, msgCopy.Payload)
	return err
}

func (ms *MsgSender) sendMessageFromBatch(msgCopy ExtInMsgCopy) error {
	if err := liteapi.VerifySendMessagePayload(msgCopy.Payload); err != nil {
		return err
	}
	for _, ch := range ms.receivers {
		ch <- msgCopy
	}
	_, err := ms.client.SendMessage(context.TODO(), msgCopy.Payload)
	return err
}

func (ms *MsgSender) SendMultipleMessages(ctx context.Context, copies []ExtInMsgCopy) {
	if len(copies) == 0 {
		return
	}
	now := time.Now().Unix()
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.batches = append(ms.batches, batchOfMessages{
		Copies: copies,
		RecvAt: now,
	})
}
