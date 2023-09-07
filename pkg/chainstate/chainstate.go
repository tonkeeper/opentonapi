package chainstate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
)

type ChainState struct {
	apy    float64
	mu     sync.RWMutex
	config config
	banned map[tongo.AccountID]struct{}
}

type config interface {
	GetLastConfig(ctx context.Context) (tlb.ConfigParams, error)
}

func (s *ChainState) GetAPY() float64 {
	return s.apy
}

func NewChainState(c config) *ChainState {
	chain := &ChainState{apy: 5.4, banned: map[tongo.AccountID]struct{}{}, config: c}

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
	c, prs := cfg.Config.Get(44)
	if !prs {
		return nil, fmt.Errorf("config doesn't have %v param", 44)
	}
	var blockedAccounts struct {
		Prefix byte
		Map    tlb.HashmapE[accID, struct{}]
	}
	err = tlb.Unmarshal(&c.Value, &blockedAccounts)
	if err != nil {
		return nil, err
	}
	m := make(map[tongo.AccountID]struct{})
	for _, a := range blockedAccounts.Map.Items() {
		m[tongo.AccountID(a.Key)] = struct{}{}
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

type accID tongo.AccountID

func (a accID) FixedSize() int {
	return 288 // (32+256) * 8
}
