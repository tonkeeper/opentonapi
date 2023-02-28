package chainstate

import (
	"encoding/json"
	"net/http"
)

type ChainState struct {
	apy float64
}

func (s *ChainState) GetAPY() float64 {
	return s.apy
}

func NewChainState() *ChainState {
	return &ChainState{apy: apyFromWhales()} //todo: replace with local calculations
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
