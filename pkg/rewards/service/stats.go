package service

import (
	"context"
	"errors"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo/liteapi"
)

const maxRounds = 101
const defaultAPY = 0.22
const poolAPYMul = 76.02

var errServiceUnavailable = errors.New("Service Unavailable")

type Stats struct {
	mu      sync.RWMutex
	options []liteapi.Option
	client  *liteapi.Client
	rounds  []core.ElectorRound
}

func NewStats(options ...liteapi.Option) *Stats {
	res := &Stats{options: options}
	var o liteapi.Options
	for _, v := range options {
		v(&o)
	}
	if len(o.LiteServers) != 0 {
		go res.updateProc()
	}
	return res
}

func (s *Stats) GetPoolAPY() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	timeNow := time.Now()
	for i := len(s.rounds); i != 0; i-- {
		if s.rounds[i-1].EndTime().Before(timeNow) {
			// latest completed round
			return s.rounds[i-1].APYOrDefault(defaultAPY) * poolAPYMul
		}
	}
	return defaultAPY * poolAPYMul
}

func (s *Stats) GetRewardsStats() (*oas.RewardsStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.rounds) == 0 {
		return nil, errServiceUnavailable
	}
	n := min(39, len(s.rounds))
	timeNow := time.Now()
	apy := make([][]float64, 0, n)
	tst := make([][]float64, 0, n)
	for _, v := range s.rounds {
		utime := float64(v.StartTime().UnixMilli())
		if v.EndTime().Before(timeNow) {
			// round is completed
			apy = append(apy, []float64{utime, v.APYOrDefault(defaultAPY)})
		}
		tst = append(tst, []float64{utime, float64(v.TotalStake)})
	}
	res := oas.RewardsStats{
		Apy:        apy,
		TotalStake: tst,
	}
	return &res, nil
}

func (s *Stats) GetStakingPoolHistory(limit int) (*oas.GetStakingPoolHistoryOK, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || len(s.rounds) < limit {
		limit = len(s.rounds)
	}
	timeNow := time.Now()
	apy := make([]oas.ApyHistory, 0, limit)
	for i := len(s.rounds); i != 0; i-- {
		v := &s.rounds[i-1]
		if v.EndTime().Before(timeNow) {
			// round is completed
			apy = append(apy, oas.ApyHistory{
				Apy:  v.APYOrDefault(defaultAPY) * poolAPYMul,
				Time: int(v.ValidatorsExt.UtimeUntil),
			})
		}
	}
	res := oas.GetStakingPoolHistoryOK{Apy: apy}
	return &res, nil
}

func (s *Stats) updateProc() {
	for {
		s.update()
		timeUntilNextUpdate := time.Minute
		s.mu.RLock()
		if len(s.rounds) != 0 {
			currentRound := s.rounds[len(s.rounds)-1]
			timeUntilNextUpdate = time.Until(currentRound.EndTime())
		}
		s.mu.RUnlock()
		<-time.After(timeUntilNextUpdate)
	}
}

func (s *Stats) update() {
	if s.client == nil {
		var err error
		s.client, err = liteapi.NewClient(s.options...)
		if err != nil {
			log.Println("rewards stats service:", err)
			return
		}
	}
	iter := core.NewElectorRoundsIterator(context.Background(), s.client)
	for v := range iter.Run {
		if !s.pushElectorRound(v) {
			break
		}
	}
	if iter.Err() != nil {
		log.Println("rewards stats service:", iter.Err())
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
