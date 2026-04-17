package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"slices"
	"sort"
	"sync"

	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"

	"github.com/tonkeeper/opentonapi/pkg/rewards/model"
)

// poolTypeCache caches pool types for addresses that don't need fresh data.
// Nominator Pools are NOT cached because we need fresh nominator data each time.
// cachedPoolInfo stores immutable pool metadata for non-Nominator-Pool types.
type cachedPoolInfo struct {
	Type                   string
	ValidatorAddress       tlb.Bits256
	OwnerAddress           tlb.MsgAddress
	ValidatorWalletAddress tlb.MsgAddress
}

type getRolesResult struct {
	OwnerAddress     tlb.MsgAddress
	ValidatorAddress tlb.MsgAddress
}

type GetPoolDataConfigTlb struct {
	ValidatorAddressBits256 tlb.Bits256
	ValidatorRewardShare    uint16
	MaxNominatorsCount      uint16
	MinValidatorStake       tlb.Coins
	MinNominatorStake       tlb.Coins
	// ds~load_uint(ADDR_SIZE()), ;; validator_address
	// ds~load_uint(16), ;; validator_reward_share
	// ds~load_uint(16), ;; max_nominators_count
	// ds~load_coins(), ;; min_validator_stake
	// ds~load_coins() ;; min_nominator_stake
}

type PackedNominatorData struct {
	Amount               tlb.Coins
	PendingDepositAmount tlb.Coins
}

type GetPoolDataTlb struct {
	State           int8      // ds~load_uint(8), ;; state
	NominatorsCount uint16    // ds~load_uint(16), ;; nominators_count
	StakeAmountSent tlb.Coins // ds~load_coins(), ;; stake_amount_sent
	ValidatorAmount tlb.Coins // ds~load_coins(), ;; validator_amount

	Config GetPoolDataConfigTlb `tlb:"^"` // unpack_config(ds~load_ref().begin_parse()), ;; config

	Nominators               tlb.Maybe[tlb.Ref[tlb.Hashmap[tlb.Bits256, PackedNominatorData]]] // ds~load_dict(), ;; nominators
	WithdrawRequests         tlb.Maybe[tlb.Ref[tlb.Hashmap[tlb.Bits256, any]]]                 // ds~load_dict(), ;; withdraw_requests
	StakeAt                  uint32                                                            // ds~load_uint(32), ;; stake_at
	SavedValidatorSetHash    tlb.Bits256                                                       // ds~load_uint(256), ;; saved_validator_set_hash
	ValidatorSetChangesCount uint8                                                             // ds~load_uint(8), ;; validator_set_changes_count
	ValidatorSetChangeTime   uint32                                                            // ds~load_uint(32), ;; validator_set_change_time
	StakeHeldFor             uint32                                                            // ds~load_uint(32), ;; stake_held_for
	ConfigProposalVotings    tlb.Maybe[tlb.Ref[tlb.Hashmap[tlb.Bits256, any]]]                 // ds~load_dict() ;; config_proposal_votings
}

var poolTypeCache sync.Map // ton.AccountID → cachedPoolInfo

// Pool type identifiers returned in the API response.
const (
	poolTypeNominatorV10       = "nominator-pool-v1.0"
	poolTypeSingleNominatorV10 = "single-nominator-pool-v1.0"
	poolTypeSingleNominatorV11 = "single-nominator-pool-v1.1"
	poolTypeOther              = "other"
)

// Known contract code hashes for deterministic pool type detection.
func mustDecodeHash(s string) [32]byte {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		panic("invalid hash: " + s)
	}
	return [32]byte(b)
}

var (
	nominatorPoolCodeHash      = mustDecodeHash("9a3ec14bc098f6b44064c305222caea2800f17dda85ee6a8198a7095ede10dcf")
	singleNominatorV10CodeHash = mustDecodeHash("a42ae69eac76ffe0e452d3d4f13d387a14e46c01a5aadba5fc1d893e6c71f5ba")
	singleNominatorV11CodeHash = mustDecodeHash("cc0d39589eb2c0cfe0fde28456657a3bdd3d953955ae3f98f25664ab3c904fbd")
)

// poolTypeByCodeHash returns the pool type for a known code hash, or "" if unknown.
func poolTypeByCodeHash(hash [32]byte) string {
	switch hash {
	case nominatorPoolCodeHash:
		return poolTypeNominatorV10
	case singleNominatorV10CodeHash:
		return poolTypeSingleNominatorV10
	case singleNominatorV11CodeHash:
		return poolTypeSingleNominatorV11
	default:
		return ""
	}
}

var pastElectionsCache struct {
	sync.Mutex
	electionIDs []int64 // sorted electAt values used as cache key
	data        map[tlb.Bits256]poolEntry
}

// poolData holds data returned by GetPoolData + ListNominators for nominator pools.
type poolData struct {
	ValidatorAmount        *big.Int
	NominatorsAmount       *big.Int
	RewardShare            uint32
	NominatorsCount        uint32
	ValidatorAddress       tlb.Bits256
	OwnerAddress           tlb.MsgAddress
	ValidatorWalletAddress tlb.MsgAddress
	Nominators             []nominatorData
}

type nominatorData struct {
	Address              tlb.Bits256
	Amount               uint64
	PendingDepositAmount uint64
	WithdrawRequested    bool
}

// fetchPoolData determines the pool type by contract code hash and, for known
// pool types, fetches enriched pool data.
//
// Detection: a single GetAccountState call retrieves the contract code hash,
// which is matched against known pool contract hashes. This is deterministic
// and avoids multiple heuristic RPC probes.
//
// All types except nominator pools are cached (they have no per-request data).
// Network errors skip detection entirely and return ("", nil) without caching.
func fetchPoolData(ctx context.Context, client LiteClient, poolAddr ton.AccountID) (string, *poolData) {
	// Fast path: confirmed type from a previous call.
	if cached, ok := poolTypeCache.Load(poolAddr); ok {
		info := cached.(cachedPoolInfo)
		if info.ValidatorAddress != (tlb.Bits256{}) || info.OwnerAddress.SumType != "" || info.ValidatorWalletAddress.SumType != "" {
			return info.Type, &poolData{
				ValidatorAddress:       info.ValidatorAddress,
				OwnerAddress:           info.OwnerAddress,
				ValidatorWalletAddress: info.ValidatorWalletAddress,
			}
		}
		return info.Type, nil
	}

	// Fetch account state to determine pool type by code hash (1 RPC call).
	st, err := retry(func() (tlb.ShardAccount, error) {
		model.CountRPC(ctx)
		return client.GetAccountState(ctx, poolAddr)
	})
	if err != nil {
		return "", nil
	}
	if st.Account.SumType != "Account" {
		return "", nil
	}
	state := st.Account.Account.Storage.State
	if state.SumType != "AccountActive" {
		return "", nil
	}
	code := state.AccountActive.StateInit.Code
	data := state.AccountActive.StateInit.Data
	if !code.Exists {
		poolTypeCache.Store(poolAddr, cachedPoolInfo{Type: poolTypeOther})
		return poolTypeOther, nil
	}
	if !data.Exists {
		poolTypeCache.Store(poolAddr, cachedPoolInfo{Type: poolTypeOther})
		return poolTypeOther, nil
	}

	codeHash, err := code.Value.Value.Hash256()
	if err != nil {
		return "", nil
	}

	poolType := poolTypeByCodeHash(codeHash)
	switch poolType {
	case poolTypeNominatorV10:
		pd := &poolData{
			// Nominators: &noms,
			NominatorsAmount: new(big.Int),
		}

		tf, raw_err := getNominatorPoolPoolData(ctx, data.Value.Value)

		if raw_err != nil {
			log.Printf("error getting nominator pool data: %v", raw_err)
			return poolType, nil
		}
		pd.ValidatorAmount = new(big.Int).SetUint64(uint64(tf.ValidatorAmount))
		pd.RewardShare = uint32(tf.Config.ValidatorRewardShare)
		pd.NominatorsCount = uint32(tf.NominatorsCount)
		pd.ValidatorAddress = tf.Config.ValidatorAddressBits256

		noms := make([]nominatorData, 0, tf.NominatorsCount)
		for _, n := range tf.Nominators.Value.Value.Items() {
			_, withdraw_requested := tf.WithdrawRequests.Value.Value.Get(n.Key)
			noms = append(noms, nominatorData{
				Address:              n.Key,
				Amount:               uint64(n.Value.Amount),
				PendingDepositAmount: uint64(n.Value.PendingDepositAmount),
				WithdrawRequested:    withdraw_requested,
			})
		}

		for _, n := range noms {
			pd.NominatorsAmount.Add(pd.NominatorsAmount, new(big.Int).SetUint64(n.Amount))
		}
		pd.Nominators = noms
		return poolType, pd

	case poolTypeSingleNominatorV10:
		// Fetch GetPoolData + fetchPoolRoles for validator/owner addresses.
		pd := &poolData{}
		if roles, err := getSingleNominatorV1011Roles(ctx, data.Value.Value); err == nil {
			pd.OwnerAddress = roles.OwnerAddress
			pd.ValidatorWalletAddress = roles.ValidatorAddress
		} else {
			log.Printf("error getting single nominator v10 roles: %v", err)
			return poolType, nil
		}
		poolTypeCache.Store(poolAddr, cachedPoolInfo{
			Type:                   poolType,
			ValidatorAddress:       pd.ValidatorAddress,
			OwnerAddress:           pd.OwnerAddress,
			ValidatorWalletAddress: pd.ValidatorWalletAddress,
		})
		return poolType, pd
	case poolTypeSingleNominatorV11:
		// Fetch GetPoolData + fetchPoolRoles for validator/owner addresses.
		pd := &poolData{}
		if roles, err := getSingleNominatorV1011Roles(ctx, data.Value.Value); err == nil {
			pd.OwnerAddress = roles.OwnerAddress
			pd.ValidatorWalletAddress = roles.ValidatorAddress
		} else {
			log.Printf("error getting single nominator v11 roles: %v", err)
			return poolType, nil
		}
		poolTypeCache.Store(poolAddr, cachedPoolInfo{
			Type:                   poolType,
			ValidatorAddress:       pd.ValidatorAddress,
			OwnerAddress:           pd.OwnerAddress,
			ValidatorWalletAddress: pd.ValidatorWalletAddress,
		})
		return poolType, pd

	default:
		// Unknown pool type — no further RPC calls.
		poolTypeCache.Store(poolAddr, cachedPoolInfo{Type: poolTypeOther})
		return poolTypeOther, nil
	}
}

func getNominatorPoolPoolData(_ context.Context, data boc.Cell) (*GetPoolDataTlb, error) {
	var tf GetPoolDataTlb
	decoder := tlb.NewDecoder()
	decoder = decoder.WithDebug()
	if err := decoder.Unmarshal(&data, &tf); err != nil {
		return nil, err
	}
	return &tf, nil
}

type singleNominatorV1011Data struct {
	OwnerAddress     tlb.MsgAddress
	ValidatorAddress tlb.MsgAddress
}

func getSingleNominatorV1011Roles(_ context.Context, data boc.Cell) (getRolesResult, error) {
	// slice := data.read
	// owner_adress = data.
	var nominator_data singleNominatorV1011Data
	if err := tlb.Unmarshal(&data, &nominator_data); err != nil {
		return getRolesResult{}, err
	}
	return getRolesResult{
		OwnerAddress:     nominator_data.OwnerAddress,
		ValidatorAddress: nominator_data.ValidatorAddress,
	}, nil

}

// extractBigInt extracts a *big.Int from a VmStackValue (VmStkTinyInt or VmStkInt).
func extractBigInt(v tlb.VmStackValue) *big.Int {
	switch v.SumType {
	case "VmStkTinyInt":
		return big.NewInt(v.VmStkTinyInt)
	case "VmStkInt":
		i := big.Int(v.VmStkInt)
		return &i
	default:
		return nil
	}
}

// frozenMember matches the TL-B layout of a member in past_elections hashmap:
// src_addr:bits256 weight:uint64 true_stake:Grams banned:Bool
type frozenMember struct {
	SrcAddr   tlb.Bits256
	Weight    tlb.Uint64
	TrueStake tlb.Grams
}

// poolEntry holds a validator's pool address and true stake from the frozen election.
type poolEntry struct {
	Addr      ton.AccountID
	TrueStake *big.Int // actual effective stake in nTON, from frozen election leaf
}

// fetchRawPastElections calls past_elections() on the elector contract and returns
// the parsed election tuples.
func fetchRawPastElections(ctx context.Context, client LiteClient, electorAddr ton.AccountID) ([]RawPastElection, error) {
	stack, err := retry(func() (tlb.VmStack, error) {
		model.CountRPC(ctx)
		_, stack, err := client.RunSmcMethod(ctx, electorAddr, "past_elections", tlb.VmStack{})
		return stack, err
	})
	if err != nil {
		return nil, fmt.Errorf("past_elections: %w", err)
	}
	if stack.Len() == 0 {
		return nil, fmt.Errorf("past_elections returned empty stack")
	}

	bottom := stack.Peek(stack.Len() - 1).VmStkTuple
	elections, err := bottom.RecursiveToSlice()
	if err != nil {
		return nil, fmt.Errorf("RecursiveToSlice: %w", err)
	}

	parsed := make([]RawPastElection, 0, len(elections))
	for _, el := range elections {
		elTuple := el.VmStkTuple
		fields, err := elTuple.Data.RecursiveToSlice(int(elTuple.Len))
		if err != nil || len(fields) < PastElFieldBonuses+1 {
			continue
		}
		pe := RawPastElection{
			ElectAt:    fields[PastElFieldElectionID].VmStkTinyInt,
			FrozenDict: fields[PastElFieldFrozenDict],
			TotalStake: extractBigInt(fields[PastElFieldTotalStake]),
			Bonuses:    extractBigInt(fields[PastElFieldBonuses]),
		}
		parsed = append(parsed, pe)
	}
	return parsed, nil
}

// getAllPoolAddresses returns a map from validator pubkey to poolEntry.
func getAllPoolAddresses(ctx context.Context, client LiteClient, electorAddr ton.AccountID) (map[tlb.Bits256]poolEntry, error) {
	parsed, err := fetchRawPastElections(ctx, client, electorAddr)
	if err != nil {
		return nil, err
	}

	// Extract election IDs (cheap) and check cache.
	ids := make([]int64, 0, len(parsed))
	for _, pe := range parsed {
		ids = append(ids, pe.ElectAt)
	}
	slices.Sort(ids)

	pastElectionsCache.Lock()
	if slices.Equal(ids, pastElectionsCache.electionIDs) && pastElectionsCache.data != nil {
		cached := pastElectionsCache.data
		pastElectionsCache.Unlock()
		log.Printf("past elections: cache hit (ids=%v)", ids)
		return cached, nil
	}
	pastElectionsCache.Unlock()

	// Cache miss — full parse of member hashmaps.
	type electionData struct {
		electAt int64
		members tlb.Hashmap[tlb.Bits256, frozenMember]
	}
	var allElections []electionData
	for _, pe := range parsed {
		membersCell := &pe.FrozenDict.VmStkCell.Value
		var members tlb.Hashmap[tlb.Bits256, frozenMember]
		if err := tlb.Unmarshal(membersCell, &members); err != nil {
			log.Printf("warning: parse members for election %d: %v", pe.ElectAt, err)
			continue
		}
		log.Printf("past election id=%d: %d members", pe.ElectAt, len(members.Keys()))
		allElections = append(allElections, electionData{electAt: pe.ElectAt, members: members})
	}

	sort.Slice(allElections, func(i, j int) bool {
		return allElections[i].electAt < allElections[j].electAt
	})

	merged := make(map[tlb.Bits256]poolEntry)
	for _, ed := range allElections {
		for _, item := range ed.members.Items() {
			merged[item.Key] = poolEntry{
				Addr:      ton.AccountID{Workchain: -1, Address: [32]byte(item.Value.SrcAddr)},
				TrueStake: new(big.Int).SetUint64(uint64(item.Value.TrueStake)),
			}
		}
	}

	pastElectionsCache.Lock()
	pastElectionsCache.electionIDs = ids
	pastElectionsCache.data = merged
	pastElectionsCache.Unlock()

	return merged, nil
}
