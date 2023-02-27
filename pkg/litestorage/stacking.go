package litestorage

import (
	"context"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

func (s *LiteStorage) GetParticipatingInWhalesPools(ctx context.Context, member tongo.AccountID) ([]core.WhalesNominator, error) {
	var result []core.WhalesNominator
	for k := range references.WhalesPools {
		_, value, err := abi.GetMembers(ctx, s.client, k)
		if err != nil {
			continue
		}
		if members, ok := value.(abi.GetMembers_WhalesNominatorResult); ok {
			for _, m := range members.Members {
				memberID, err := tongo.AccountIDFromTlb(m.Address)
				if err != nil {
					continue
				}
				if memberID != nil && member == *memberID {
					result = append(result, core.WhalesNominator{
						Pool:                  k,
						Member:                member,
						MemberBalance:         m.MemberBalance,
						MemberPendingDeposit:  m.MemberPendingDeposit,
						MemberPendingWithdraw: m.MemberPendingWithdraw,
						MemberWithdraw:        m.MemberWithdraw,
					})
				}
			}
		}
	}
	return result, nil
}
