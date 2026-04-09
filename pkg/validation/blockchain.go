package validation

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

// electorAddr is the well-known address of the TON elector contract (-1:333...333).
var electorAddr = ton.AccountID{
	Workchain: -1,
	Address: [32]byte{
		0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33,
		0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33,
		0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33,
		0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33,
	},
}

// past_elections tuple layout from elector-code.fc:
// [election_id, unfreeze_at, stake_held, vset_hash, frozen_dict, total_stake, bonuses, complaints]
// https://github.com/ton-blockchain/ton/blob/master/crypto/smartcont/elector-code.fc
const (
	PastElFieldElectionID = 0
	PastElFieldFrozenDict = 4
	PastElFieldTotalStake = 5
	PastElFieldBonuses    = 6
)

// RawPastElection holds a single parsed past_elections tuple from the elector contract.
type RawPastElection struct {
	ElectAt    int64
	FrozenDict tlb.VmStackValue // cell: hashmap of frozen members
	TotalStake *big.Int         // nil if not available
	Bonuses    *big.Int         // nil if not available
}

// lookupMasterchainBlock resolves a seqno to a BlockIDExt and returns the block time.
func lookupMasterchainBlock(ctx context.Context, client *liteapi.Client, seqno uint32) (ton.BlockIDExt, time.Time, error) {
	blockID := ton.BlockID{
		Workchain: -1,
		Shard:     0x8000000000000000,
		Seqno:     seqno,
	}
	ext, info, err := client.LookupBlock(ctx, blockID, 1, nil, nil)
	if err != nil {
		return ton.BlockIDExt{}, time.Time{}, err
	}
	return ext, time.Unix(int64(info.GenUtime), 0), nil
}

// getRoundInfo returns the unix timestamps of the current validation round start and end.
// Config param 34 has two TL-B variants: "validators#11" (legacy) and "validators_ext#12"
// (current, adds TotalWeight). Both carry UtimeSince/UtimeUntil so we handle both.
func getRoundInfo(conf *ton.BlockchainConfig) (since, until uint32) {
	if conf.ConfigParam34 == nil {
		return
	}
	vs := conf.ConfigParam34.CurValidators
	switch vs.SumType {
	case "Validators":
		since = vs.Validators.UtimeSince
		until = vs.Validators.UtimeUntil
	case "ValidatorsExt":
		since = vs.ValidatorsExt.UtimeSince
		until = vs.ValidatorsExt.UtimeUntil
	}
	return since, until
}

// lookupMasterchainBlockByUtime resolves a unix timestamp to the nearest masterchain block.
func lookupMasterchainBlockByUtime(ctx context.Context, client *liteapi.Client, utime uint32) (ton.BlockIDExt, error) {
	blockID := ton.BlockID{
		Workchain: -1,
		Shard:     0x8000000000000000,
	}
	ext, _, err := client.LookupBlock(ctx, blockID, 4, nil, &utime)
	return ext, err
}

// computeReturnedStake calls the elector's compute_returned_stake(addr) get method.
// Returns the credits balance for the given address.
func computeReturnedStake(ctx context.Context, client *liteapi.Client, addr ton.AccountID) (*big.Int, error) {
	addrInt := new(big.Int).SetBytes(addr.Address[:])
	param := tlb.VmStackValue{
		SumType:  "VmStkInt",
		VmStkInt: tlb.Int257(*addrInt),
	}
	_, stack, err := client.RunSmcMethod(ctx, electorAddr, "compute_returned_stake", tlb.VmStack{param})
	if err != nil {
		return nil, fmt.Errorf("compute_returned_stake(%s): %w", addr.ToRaw(), err)
	}
	if len(stack) == 0 {
		return nil, fmt.Errorf("compute_returned_stake(%s): empty stack", addr.ToRaw())
	}
	val := extractBigInt(stack[0])
	if val == nil {
		return nil, fmt.Errorf("compute_returned_stake(%s): unexpected stack type %s", addr.ToRaw(), stack[0].SumType)
	}
	return val, nil
}

// extractValidators returns all ValidatorDescr entries from config param 34.
// Handles both "validators#11" and "validators_ext#12" TL-B variants.
func extractValidators(conf *ton.BlockchainConfig) []tlb.ValidatorDescr {
	if conf.ConfigParam34 == nil {
		return nil
	}
	vs := conf.ConfigParam34.CurValidators
	var items []tlb.ValidatorDescr
	switch vs.SumType {
	case "Validators":
		for _, item := range vs.Validators.List.Items() {
			items = append(items, item.Value)
		}
	case "ValidatorsExt":
		for _, item := range vs.ValidatorsExt.List.Items() {
			items = append(items, item.Value)
		}
	}
	return items
}
