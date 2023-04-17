package chainstate

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type ChainState struct {
	apy float64
	mu  sync.RWMutex
}

func (s *ChainState) GetAPY() float64 {
	return s.apy
}

func NewChainState() *ChainState {
	chain := &ChainState{apy: apyFromWhales()} //todo: replace with local calculations

	go func() {
		for {
			chain.refresh()
			time.Sleep(time.Minute * 30)
		}
	}()

	return chain
}

func (s *ChainState) refresh() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apy = apyFromWhales()
}

func apyFromWhales() float64 {
	v := 6.84
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
		v = body.Apy
		break
	}
	return v
}
