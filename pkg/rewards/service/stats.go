package service

import (
	"context"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo/liteapi"
)

const maxRounds = 39

type Stats struct {
	mu     sync.RWMutex
	client *liteapi.Client
	rounds []core.ElectorRound
}

func NewStats(client *liteapi.Client) *Stats {
	s := &Stats{client: client}
	go s.updateProc()
	return s
}

func (s *Stats) GetStats() oas.RewardsStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.rounds) == 0 {
		return oas.RewardsStats{}
	}
	apy := make([][]float64, 0, len(s.rounds))
	tst := make([][]float64, 0, len(s.rounds))
	for _, round := range s.rounds {
		utime := float64(round.StartTime().UnixMilli())
		apy = append(apy, []float64{utime, round.APY()})
		tst = append(tst, []float64{utime, float64(round.TotalStake)})
	}
	// Don't show APY for current round - it is not finalized yet.
	apy = apy[:len(s.rounds)-1]
	return oas.RewardsStats{
		Apy:        apy,
		TotalStake: tst,
	}
}

func (s *Stats) updateProc() {
	s.update()
	for range time.Tick(5 * time.Minute) {
		s.update()
	}
}

func (s *Stats) update() {
	iter := core.NewElectorRoundsIterator(context.Background(), s.client)
	for v := range iter.Run {
		if !s.pushElectorRound(v) {
			break
		}
	}
	if iter.Err() != nil {
		log.Println("StatsService:", iter.Err())
	}
}

func (s *Stats) pushElectorRound(v core.ElectorRound) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := len(s.rounds)
	for ; i != 0; i-- {
		if s.rounds[i-1].ID() == v.ID() {
			s.rounds[i-1] = v
			return false
		}
		if s.rounds[i-1].StartTime().Before(v.StartTime()) {
			break
		}
	}
	full := len(s.rounds) == maxRounds
	if i == 0 && full {
		return false
	}
	s.rounds = slices.Insert(s.rounds, i, v)
	if full {
		s.rounds = slices.Delete(s.rounds, 0, 1)
	}
	return true
}
