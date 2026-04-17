package api

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	gocoon "github.com/tonkeeper/gocoon/pkg/client"
	"go.uber.org/zap"
)

const defaultCocoonPoolRefreshInterval = 30 * time.Second

type CocoonDialer interface {
	Connect(ctx context.Context, logger *zap.Logger) (*gocoon.Connection, error)
}

type CocoonPool struct {
	mu              sync.RWMutex
	size            int
	client          CocoonDialer
	logger          *zap.Logger
	conns           []*gocoon.Connection
	refreshInterval time.Duration
}

func NewCocoonPool(client CocoonDialer, logger *zap.Logger, size int, refreshInterval time.Duration) *CocoonPool {
	if size <= 0 {
		size = 3
	}
	if refreshInterval <= 0 {
		refreshInterval = defaultCocoonPoolRefreshInterval
	}
	return &CocoonPool{
		client:          client,
		logger:          logger,
		size:            size,
		conns:           make([]*gocoon.Connection, size),
		refreshInterval: refreshInterval,
	}
}

func (p *CocoonPool) StartAsync() {
	for slot := range p.size {
		go p.fillSlot(slot)
	}
	if p.logger != nil {
		p.logger.Info("cocoon pool: periodic connection refresh", zap.Duration("interval", p.refreshInterval))
	}
	go p.refreshLoop()
}

func (p *CocoonPool) refreshLoop() {
	t := time.NewTicker(p.refreshInterval)
	defer t.Stop()
	for range t.C {
		p.mu.Lock()
		for i := range p.conns {
			if c := p.conns[i]; c != nil {
				_ = c.Close()
				p.conns[i] = nil
			}
		}
		p.mu.Unlock()
		if p.logger != nil {
			p.logger.Info("cocoon pool: closing all connections for refresh")
		}
		for slot := range p.size {
			go p.fillSlot(slot)
		}
	}
}

func (p *CocoonPool) fillSlot(slot int) {
	ctx := context.Background()
	conn, err := p.client.Connect(ctx, p.logger)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("cocoon pool: connect failed", zap.Int("slot", slot), zap.Error(err))
		}
		return
	}
	p.mu.Lock()
	if old := p.conns[slot]; old != nil {
		_ = old.Close()
	}
	p.conns[slot] = conn
	p.mu.Unlock()
	if p.logger != nil {
		p.logger.Info("cocoon pool: connection ready", zap.Int("slot", slot))
	}
}

func (p *CocoonPool) pick(ctx context.Context) (*gocoon.Connection, error) {
	p.mu.RLock()
	ready := make([]*gocoon.Connection, 0, p.size)
	for _, c := range p.conns {
		if c != nil {
			ready = append(ready, c)
		}
	}
	p.mu.RUnlock()
	if len(ready) > 0 {
		return ready[rand.IntN(len(ready))], nil
	}
	return p.client.Connect(ctx, p.logger)
}
