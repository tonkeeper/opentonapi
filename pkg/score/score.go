package score

import (
	"github.com/tonkeeper/tongo/ton"
)

type Score struct{}

func NewScore() *Score {
	return &Score{}
}

func (s *Score) GetJettonScore(masterID ton.AccountID) (int32, error) {
	return 0, nil
}
