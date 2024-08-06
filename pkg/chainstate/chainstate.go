package chainstate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/ton"
)

type ChainState struct {
	apy    float64
	mu     sync.RWMutex
	config config
	banned map[tongo.AccountID]struct{}
}

type config interface {
	GetLastConfig(ctx context.Context) (ton.BlockchainConfig, error)
}

func (s *ChainState) GetAPY() float64 {
	return s.apy
}

func NewChainState(c config) *ChainState {
	chain := &ChainState{apy: 3.3, banned: map[tongo.AccountID]struct{}{}, config: c}

	go func() {
		for {
			chain.refresh()
			time.Sleep(time.Minute * 30)
		}
	}()

	return chain
}

func (s *ChainState) refresh() {
	apy, err1 := apyFromWhales()
	banned, err2 := suspended(s.config)
	s.mu.Lock()
	if err1 == nil {
		s.apy = apy
	}
	if err2 == nil {
		s.banned = banned
	}

	s.mu.Unlock()
}

func suspended(conf config) (map[tongo.AccountID]struct{}, error) {
	cfg, err := conf.GetLastConfig(context.TODO())
	if err != nil {
		return nil, err
	}
	// optimization to avoid processing all config keys

	if cfg.ConfigParam44 == nil {
		return nil, fmt.Errorf("config doesn't have %v param", 44)
	}
	m := make(map[tongo.AccountID]struct{}, len(cfg.ConfigParam44.SuspendedAddressList.Addresses.Keys()))
	for _, addr := range cfg.ConfigParam44.SuspendedAddressList.Addresses.Keys() {
		accountID := ton.AccountID{
			Workchain: int32(addr.Workchain),
			Address:   addr.Address,
		}
		m[accountID] = struct{}{}
	}
	return m, nil
}

func apyFromWhales() (float64, error) { //todo: replace with local calculations
	for i := 0; i < 5; i++ {
		resp, err := http.Get("https://connect.tonhubapi.com/net/mainnet/elections/latest/apy")
		if err != nil {
			continue
		}
		var body struct {
			Apy float64 `json:"apy,string"`
		}
		err = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if err != nil || body.Apy == 0 {
			continue
		}
		return body.Apy, nil
	}
	return 0, fmt.Errorf("not found")
}

func (s *ChainState) CheckIsSuspended(id tongo.AccountID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, prs := s.banned[id]
	return prs
}
