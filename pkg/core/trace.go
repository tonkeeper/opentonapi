package core

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/tonkeeper/tongo/ton"
	"sync"

	"github.com/shopspring/decimal"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"golang.org/x/exp/maps"
)

var (
	ErrTraceIsTooLong = errors.New("trace is too long")
)

// TraceID identifies a trace by a hash of the transaction which created it and the logical time of the transaction.
type TraceID struct {
	Hash  tongo.Bits256
	Lt    uint64
	UTime int64
}

type Trace struct {
	// Transaction is slightly modified.
	// For example, we have kept only external outbound messages in OutMsgs.
	Transaction
	AccountInterfaces []abi.ContractInterface
	Children          []*Trace

	// mu protects "additionalInfo" field.
	mu sync.RWMutex
	// additionalInfo holds information about this trace.
	// It is protected by a mutex because we cache traces and set additionalInfo independently of the trace itself.
	// so it happens that two different goroutines get a trace from the cache and attempt to set additionalInfo.
	additionalInfo *TraceAdditionalInfo
}

// TraceAdditionalInfo holds information about a trace
// but not directly extracted from it or a corresponding transaction.
type TraceAdditionalInfo struct {
	// JettonMasters maps jetton wallets to their masters.
	JettonMasters map[tongo.AccountID]tongo.AccountID
	// NftSaleContract is set, if a transaction's account implements "get_sale_data" method.
	NftSaleContract *NftSaleContract
	// STONfiPool is set, if a transaction's account implements "get_pool_data" method and abi.StonfiPool interface.
	STONfiPool *STONfiPool

	// EmulatedTeleitemNFT is set, if this trace is a result of emulation.
	// This field is required because when a new NFT is created during emulation,
	// there is no way to get it from the blockchain, and we have to store it somewhere.
	EmulatedTeleitemNFT *EmulatedTeleitemNFT
}

func (t *Trace) AdditionalInfo() *TraceAdditionalInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.additionalInfo
}

func (t *Trace) SetAdditionalInfo(info *TraceAdditionalInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.additionalInfo = info
}

func (t *TraceAdditionalInfo) MarshalJSON() ([]byte, error) {
	type Alias struct {
		JettonMasters       map[string]string    `json:",omitempty"`
		NftSaleContract     *NftSaleContract     `json:",omitempty"`
		STONfiPool          *STONfiPool          `json:",omitempty"`
		EmulatedTeleitemNFT *EmulatedTeleitemNFT `json:",omitempty"`
	}

	masters := make(map[string]string)
	if t.JettonMasters != nil {
		for k, v := range t.JettonMasters {
			masters[k.String()] = v.String()
		}
	}

	return json.Marshal(&Alias{
		JettonMasters:       masters,
		NftSaleContract:     t.NftSaleContract,
		STONfiPool:          t.STONfiPool,
		EmulatedTeleitemNFT: t.EmulatedTeleitemNFT,
	})
}

func (t *TraceAdditionalInfo) UnmarshalJSON(data []byte) error {
	type Alias struct {
		JettonMasters       map[string]string    `json:",omitempty"`
		NftSaleContract     *NftSaleContract     `json:",omitempty"`
		STONfiPool          *STONfiPool          `json:",omitempty"`
		EmulatedTeleitemNFT *EmulatedTeleitemNFT `json:",omitempty"`
	}

	aux := &Alias{}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	if aux.JettonMasters != nil {
		t.JettonMasters = make(map[tongo.AccountID]tongo.AccountID)
		for kStr, vStr := range aux.JettonMasters {
			key := ton.MustParseAccountID(kStr)
			val := ton.MustParseAccountID(vStr)
			t.JettonMasters[key] = val
		}
	}

	t.NftSaleContract = aux.NftSaleContract
	t.STONfiPool = aux.STONfiPool
	t.EmulatedTeleitemNFT = aux.EmulatedTeleitemNFT

	return nil
}

func (t *Trace) InProgress() bool {
	return t.countUncompleted() != 0
}
func (t *Trace) countUncompleted() int {
	c := 0
	for i := range t.OutMsgs {
		if t.OutMsgs[i].Destination != nil {
			c++
		}
	}
	for _, st := range t.Children {
		c += st.countUncompleted()
	}
	return c
}

type EmulatedTeleitemNFT struct {
	Index             decimal.Decimal
	CollectionAddress *tongo.AccountID
	Verified          bool
}

// NftSaleContract holds partial results of get_sale_data method.
type NftSaleContract struct {
	NftPrice int64
	// Owner of an NFT according to a getgems/basic contract.
	Owner *tongo.AccountID
	Item  tongo.AccountID
}

// STONfiPool holds partial results of execution of STONfi's "get_pool_data" method.
type STONfiPool struct {
	Token0 tongo.AccountID
	Token1 tongo.AccountID
}

type STONfiVersion string

const (
	STONfiPoolV1 STONfiVersion = "v1"
	STONfiPoolV2 STONfiVersion = "v2"
)

type STONfiPoolID struct {
	ID      tongo.AccountID
	Version STONfiVersion
}

// InformationSource provides methods to construct TraceAdditionalInfo.
type InformationSource interface {
	JettonMastersForWallets(ctx context.Context, wallets []tongo.AccountID) (map[tongo.AccountID]tongo.AccountID, error)
	NftSaleContracts(ctx context.Context, contracts []tongo.AccountID) (map[tongo.AccountID]NftSaleContract, error)
	STONfiPools(ctx context.Context, poolIDs []STONfiPoolID) (map[tongo.AccountID]STONfiPool, error)
}

func isDestinationJettonWallet(inMsg *Message) bool {
	if inMsg == nil || inMsg.DecodedBody == nil {
		return false
	}
	return (inMsg.DecodedBody.Operation == abi.JettonTransferMsgOp ||
		inMsg.DecodedBody.Operation == abi.JettonInternalTransferMsgOp ||
		inMsg.DecodedBody.Operation == abi.JettonBurnMsgOp) && inMsg.Destination != nil
}

func hasInterface(interfacesList []abi.ContractInterface, name abi.ContractInterface) bool {
	for _, iface := range interfacesList {
		if iface.Implements(name) {
			return true
		}
	}
	return false
}

func Visit(trace *Trace, fn func(trace *Trace)) {
	fn(trace)
	for _, child := range trace.Children {
		Visit(child, fn)
	}
}

// DistinctAccounts returns a list of accounts that are involved in the given trace.
func DistinctAccounts(trace *Trace) []tongo.AccountID {
	accounts := make(map[tongo.AccountID]struct{})
	Visit(trace, func(trace *Trace) {
		accounts[trace.Account] = struct{}{}
	})
	return maps.Keys(accounts)
}

// CollectAdditionalInfo goes over the whole trace
// and populates trace.TraceAdditionalInfo based on information
// provided by InformationSource.
func CollectAdditionalInfo(ctx context.Context, infoSource InformationSource, trace *Trace) error {
	if infoSource == nil {
		return nil
	}
	var jettonWallets []tongo.AccountID
	var saleContracts []tongo.AccountID
	var stonfiPoolIDs []STONfiPoolID
	Visit(trace, func(trace *Trace) {
		// when we emulate a trace,
		// we construct "trace.AdditionalInfo" in emulatedTreeToTrace for all accounts the trace touches.
		// moreover, some accounts change their states and some of them are not exist in the blockchain,
		// so we must not inspect them again.
		if trace.AdditionalInfo() != nil {
			return
		}
		if isDestinationJettonWallet(trace.InMsg) {
			jettonWallets = append(jettonWallets, *trace.InMsg.Destination)
		}
		if hasInterface(trace.AccountInterfaces, abi.NftSaleV1) ||
			hasInterface(trace.AccountInterfaces, abi.NftSaleV2) ||
			hasInterface(trace.AccountInterfaces, abi.NftAuctionV1) {
			saleContracts = append(saleContracts, trace.Account)
		}
		if hasInterface(trace.AccountInterfaces, abi.StonfiPool) {
			stonfiPoolIDs = append(stonfiPoolIDs, STONfiPoolID{ID: trace.Account, Version: STONfiPoolV1})
		}
		if hasInterface(trace.AccountInterfaces, abi.StonfiPoolV2) {
			stonfiPoolIDs = append(stonfiPoolIDs, STONfiPoolID{ID: trace.Account, Version: STONfiPoolV2})
		}
	})
	stonfiPools, err := infoSource.STONfiPools(ctx, stonfiPoolIDs)
	if err != nil {
		return err
	}
	for _, pool := range stonfiPools {
		jettonWallets = append(jettonWallets, pool.Token0)
		jettonWallets = append(jettonWallets, pool.Token1)
	}
	masters, err := infoSource.JettonMastersForWallets(ctx, jettonWallets)
	if err != nil {
		return err
	}
	basicNftSales, err := infoSource.NftSaleContracts(ctx, saleContracts)
	if err != nil {
		return err
	}
	Visit(trace, func(trace *Trace) {
		// when we emulate a trace,
		// we construct "trace.AdditionalInfo" in emulatedTreeToTrace for all accounts the trace touches.
		// moreover, some accounts change their states and some of them are not exist in the blockchain,
		// so we must not inspect them again.
		if trace.AdditionalInfo() != nil {
			return
		}
		additionalInfo := &TraceAdditionalInfo{}
		if isDestinationJettonWallet(trace.InMsg) {
			if master, ok := masters[*trace.InMsg.Destination]; ok {
				additionalInfo.SetJettonMaster(*trace.InMsg.Destination, master)
			}
		}
		if hasInterface(trace.AccountInterfaces, abi.NftSaleV1) ||
			hasInterface(trace.AccountInterfaces, abi.NftSaleV2) ||
			hasInterface(trace.AccountInterfaces, abi.NftAuctionV1) {
			if sale, ok := basicNftSales[trace.Account]; ok {
				additionalInfo.NftSaleContract = &sale
			}
		}
		if hasInterface(trace.AccountInterfaces, abi.StonfiPool) || hasInterface(trace.AccountInterfaces, abi.StonfiPoolV2) {
			if pool, ok := stonfiPools[trace.Account]; ok {
				additionalInfo.STONfiPool = &pool
				additionalInfo.SetJettonMaster(pool.Token0, masters[pool.Token0])
				additionalInfo.SetJettonMaster(pool.Token1, masters[pool.Token1])
			}
		}
		trace.SetAdditionalInfo(additionalInfo)
	})
	return nil
}

func (info *TraceAdditionalInfo) JettonMaster(jettonWallet tongo.AccountID) (tongo.AccountID, bool) {
	if info.JettonMasters == nil {
		return tongo.AccountID{}, false
	}
	master, ok := info.JettonMasters[jettonWallet]
	return master, ok
}

func (info *TraceAdditionalInfo) SetJettonMaster(jettonWallet tongo.AccountID, jettonMaster tongo.AccountID) {
	if info.JettonMasters == nil {
		info.JettonMasters = make(map[tongo.AccountID]tongo.AccountID)
	}
	info.JettonMasters[jettonWallet] = jettonMaster
}
