package litestorage

import (
	"context"
	"fmt"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo/ton"
)

func (s *LiteStorage) GetJettonTransferPayload(ctx context.Context, accountID, jettonMaster ton.AccountID) (*core.JettonTransferPayload, error) {
	return nil, fmt.Errorf("not implemented")
}
