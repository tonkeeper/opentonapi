package api

import (
	"fmt"

	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/contract/elector"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

// Elector storage layout (governance-contract/elector-code.fc):
//
//	store_data:
//	  elect:          Maybe ^Elect
//	  credits:        HashmapE 256 Coins
//	  past_elections: HashmapE 32  PastElection
//	  grams:          Coins
//	  active_id:      uint32
//	  active_hash:    uint256
//
//	Elect:
//	  elect_at:32 elect_close:32 min_stake:Coins total_stake:Coins
//	  members:HashmapE 256 Member failed:int1 finished:int1
//
//	Member:
//	  stake:Coins time:32 max_factor:32 src_addr:uint256 adnl_addr:uint256

type electorMember struct {
	Stake     tlb.Grams
	Time      uint32
	MaxFactor uint32
	SrcAddr   tlb.Bits256
	AdnlAddr  tlb.Bits256
}

type electorElectData struct {
	ElectAt    uint32
	ElectClose uint32
	MinStake   tlb.Grams
	TotalStake tlb.Grams
	Members    tlb.HashmapE[tlb.Bits256, electorMember]
}

type electorStorage struct {
	Elect tlb.Maybe[tlb.Ref[electorElectData]]
}

// parseElectorParticipantList decodes the elector contract storage and returns
// the participant list of the currently active election. This bypasses the
// participant_list_extended get-method, whose result cannot be serialized by
// the TVM emulator once the cons-list of participants exceeds the cell-depth
// limit (~512).
func parseElectorParticipantList(data *boc.Cell) (*elector.ParticipantList, error) {
	if data == nil {
		return nil, fmt.Errorf("elector data is nil")
	}
	data.ResetCounters()
	var storage electorStorage
	if err := tlb.Unmarshal(data, &storage); err != nil {
		return nil, fmt.Errorf("decode elector storage: %w", err)
	}
	if !storage.Elect.Exists {
		// Matches the get-method behaviour: when there is no active
		// election, return an empty list with ElectAt=0 so the caller's
		// per-key-block loop walks further back in time.
		return &elector.ParticipantList{}, nil
	}
	elect := storage.Elect.Value.Value
	items := elect.Members.Items()
	validators := make([]elector.Validator, 0, len(items))
	for _, item := range items {
		m := item.Value
		validators = append(validators, elector.Validator{
			Pubkey:    item.Key,
			Stake:     int64(m.Stake),
			MaxFactor: int64(m.MaxFactor),
			Address:   ton.AccountID{Workchain: -1, Address: m.SrcAddr},
			AdnlAddr:  m.AdnlAddr.Hex(),
		})
	}
	return &elector.ParticipantList{
		ElectAt:    int64(elect.ElectAt),
		ElectClose: int64(elect.ElectClose),
		MinStake:   int64(elect.MinStake),
		TotalStake: int64(elect.TotalStake),
		Validators: validators,
	}, nil
}
