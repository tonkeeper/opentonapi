package litestorage

import (
	"context"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
)

func (s *LiteStorage) GetLogs(ctx context.Context, account tongo.AccountID, destination *tlb.MsgAddress, limit int, beforeLT uint64) ([]core.Message, error) {
	var messages []core.Message
	if beforeLT == 0 {
		beforeLT = 1 << 63
	}
	txLt := beforeLT
	for {
		txs, err := s.GetAccountTransactions(ctx, account, limit, txLt, 0, true)
		if err != nil {
			return nil, err
		}
		if len(txs) == 0 {
			return messages, nil
		}
		for _, tx := range txs {
			txLt = tx.Lt
			for _, m := range tx.OutMsgs {
				if m.CreatedLt >= beforeLT {
					continue
				}
				if m.Destination != nil {
					continue
				}
				if destination != nil {
					if destination.SumType == "AddrNone" && m.DestinationExtern != nil {
						continue
					}
					if destination.SumType == "AddrExtern" && (m.DestinationExtern == nil || m.DestinationExtern.ToFiftHex() != destination.AddrExtern.ExternalAddress.ToFiftHex()) {
						continue
					}
				}
				messages = append(messages, m)
				if len(messages) == limit {
					return messages, nil
				}
			}
		}
		if len(txs) < limit {
			return messages, nil
		}
	}
}
