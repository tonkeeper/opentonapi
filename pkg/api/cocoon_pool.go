package api

import (
	"context"
	"math/rand/v2"
	"sync"

	gocoon "github.com/tonkeeper/gocoon/pkg/client"
	"go.uber.org/zap"
)

type CocoonDialer interface {
	Connect(ctx context.Context, logger *zap.Logger) (*gocoon.Connection, error)
}

type CocoonPool struct {
	mu     sync.Mutex
	size   int
	client CocoonDialer
	logger *zap.Logger
	conns  []*gocoon.Connection
}

func NewCocoonPool(client CocoonDialer, logger *zap.Logger, size int) *CocoonPool {
	if size <= 0 {
		size = 3
	}
	return &CocoonPool{
		client: client,
		logger: logger,
		size:   size,
		conns:  make([]*gocoon.Connection, size),
	}
}

func (p *CocoonPool) StartAsync() {
	for slot := range p.size {
		go func(slot int) {
			ctx := context.Background()
			conn, err := p.client.Connect(ctx, p.logger)
			if err != nil {
				if p.logger != nil {
					p.logger.Warn("cocoon pool: connect failed", zap.Int("slot", slot), zap.Error(err))
				}
				return
			}
			p.mu.Lock()
			p.conns[slot] = conn
			p.mu.Unlock()
			if p.logger != nil {
				p.logger.Info("cocoon pool: connection ready", zap.Int("slot", slot))
			}
		}(slot)
	}
}

func (p *CocoonPool) pick(ctx context.Context) (*gocoon.Connection, error) {
	p.mu.Lock()
	ready := make([]*gocoon.Connection, 0, p.size)
	for _, c := range p.conns {
		if c != nil {
			ready = append(ready, c)
		}
	}
	p.mu.Unlock()
	if len(ready) > 0 {
		return ready[rand.IntN(len(ready))], nil
	}
	return p.client.Connect(ctx, p.logger)
}
