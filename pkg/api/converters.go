package api

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/go-faster/jx"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

func blockIdFromString(s string) (tongo.BlockID, error) {
	var id tongo.BlockID
	_, err := fmt.Sscanf(s, "(%d,%x,%d)", &id.Workchain, &id.Shard, &id.Seqno)
	if err != nil {
		return tongo.BlockID{}, err
	}
	return id, nil
}

func convertBlockHeader(b core.BlockHeader) oas.Block {
	res := oas.Block{
		WorkchainID:       b.Workchain,
		Shard:             fmt.Sprintf("%x", b.Shard),
		Seqno:             int32(b.Seqno),
		RootHash:          fmt.Sprintf("%x", b.RootHash),
		FileHash:          fmt.Sprintf("%x", b.FileHash),
		GlobalID:          b.GlobalId,
		Version:           int32(b.Version),
		AfterMerge:        b.AfterMerge,
		BeforeSplit:       b.BeforeSplit,
		AfterSplit:        b.AfterSplit,
		WantSplit:         b.WantSplit,
		WantMerge:         b.WantMerge,
		KeyBlock:          b.IsKeyBlock,
		GenUtime:          int64(b.GenUtime),
		StartLt:           b.StartLt,
		EndLt:             b.EndLt,
		VertSeqno:         int32(b.VertSeqno),
		GenCatchainSeqno:  b.CatchainSeqno,
		MinRefMcSeqno:     b.MinRefMcSeqno,
		PrevKeyBlockSeqno: b.PrevKeyBlockSeqno,
		InMsgDescrLength:  int64(b.BlockExtra.InMsgDescrLength),
		OutMsgDescrLength: int64(b.BlockExtra.OutMsgDescrLength),
		RandSeed:          fmt.Sprintf("%x", b.BlockExtra.RandSeed),
		CreatedBy:         fmt.Sprintf("%x", b.BlockExtra.CreatedBy),
	}
	if b.GenSoftware != nil {
		res.GenSoftwareVersion.SetTo(int32(b.GenSoftware.Version))
		res.GenSoftwareCapabilities.SetTo(int64(b.GenSoftware.Capabilities))
	}
	if b.MasterRef != nil {
		res.MasterRef.SetTo(b.MasterRef.BlockID.String())
	}
	for _, r := range b.PrevBlocks {
		res.PrevRefs = append(res.PrevRefs, r.BlockID.String())
	}
	return res
}

func convertTransaction(t core.Transaction) oas.Transaction {
	tx := oas.Transaction{
		Hash:            t.Hash.Hex(),
		Lt:              int64(t.Lt),
		Account:         convertAccountAddress(t.Account),
		Success:         t.Success,
		Utime:           t.Utime,
		OrigStatus:      oas.AccountStatus(t.OrigStatus),
		EndStatus:       oas.AccountStatus(t.EndStatus),
		TotalFees:       t.Fee,
		TransactionType: oas.TransactionType(t.Type),
		StateUpdateOld:  t.StateHashUpdate.OldHash.Hex(),
		StateUpdateNew:  t.StateHashUpdate.NewHash.Hex(),
		Block:           t.BlockID.String(),
		//ComputePhase:    oas.OptComputePhase{}, //TODO: write
		//StoragePhase:    oas.OptStoragePhase{},
		//CreditPhase:     oas.OptCreditPhase{},
		//ActionPhase:     oas.OptActionPhase{},
		//BouncePhase:     oas.OptBouncePhaseType{},
		Aborted:   t.Aborted,
		Destroyed: t.Destroyed,
	}
	if t.PrevTransLt != 0 {
		tx.PrevTransLt.Value = int64(t.PrevTransLt)
		tx.PrevTransLt.Set = true
		tx.PrevTransHash.Value = t.PrevTransHash.Hex()
		tx.PrevTransHash.Set = true
	}
	if t.InMsg != nil {
		tx.InMsg.SetTo(convertMessage(*t.InMsg))
	}
	for _, m := range t.OutMsgs {
		tx.OutMsgs = append(tx.OutMsgs, convertMessage(m))
	}
	return tx
}

func convertMessage(m core.Message) oas.Message {
	msg := oas.Message{
		CreatedLt:   int64(m.CreatedLt),
		IhrDisabled: m.IhrDisabled,
		Bounce:      m.Bounce,
		Bounced:     m.Bounced,
		Value:       m.Value,
		FwdFee:      m.FwdFee,
		IhrFee:      m.IhrFee,
		Destination: convertOptAccountAddress(m.Destination),
		Source:      convertOptAccountAddress(m.Source),
		ImportFee:   m.ImportFee,
		CreatedAt:   int64(m.CreatedAt),
		DecodedBody: nil,
	}
	if m.OpCode != nil {
		msg.OpCode = oas.NewOptString("0x" + hex.EncodeToString(binary.BigEndian.AppendUint32(nil, *m.OpCode)))
	}
	if len(m.Init) != 0 {
		//todo: return init
		//cells, err := boc.DeserializeBoc(m.Init)
		//if err == nil && len(cells) == 1 {
		//	var stateInit tlb.StateInit
		//	err = tlb.Unmarshal(cells[0], &stateInit)
		//
		//}
	}
	if m.DecodedBody != nil {
		msg.DecodedOpName = oas.NewOptString(m.DecodedBody.Operation)
		msg.DecodedBody = anyToJSONRawMap(m.DecodedBody.Value)
	}
	return msg
}

func convertTrace(t core.Trace) oas.Trace {
	trace := oas.Trace{Transaction: convertTransaction(t.Transaction)}
	for _, c := range t.Children {
		trace.Children = append(trace.Children, convertTrace(*c))
	}
	return trace
}

func convertStackingWhalesPool(address tongo.AccountID, w references.WhalesPoolInfo, poolStatus abi.GetStakingStatusResult, poolConfig abi.GetParams_WhalesNominatorResult, apy float64) oas.PoolInfo {
	return oas.PoolInfo{
		Address:        address.ToRaw(),
		Name:           w.Name + " " + w.Queue,
		TotalAmount:    int64(poolStatus.StakeSent),
		Implementation: oas.PoolInfoImplementationWhales,
		Apy:            apy * float64(10000-poolConfig.PoolFee) / 10000,
		MinStake:       poolConfig.MinStake,
		CycleEnd:       int64(poolStatus.StakeUntil),
		CycleStart:     int64(poolStatus.StakeAt),
	}
}

func convertNFT(item core.NftItem) oas.NftItem {
	return oas.NftItem{
		Address:    item.Address.ToRaw(),
		Index:      item.Index.BigInt().Int64(),
		Owner:      convertOptAccountAddress(item.OwnerAddress),
		Collection: oas.OptNftItemCollection{}, //todo: add
		Verified:   item.Verified,
		Metadata:   anyToJSONRawMap(item.Metadata),
		Sale:       oas.OptSale{}, //todo: add
		Previews:   nil,           //todo: add
		DNS:        pointerToOptString(item.DNS),
		ApprovedBy: nil, //todo: add
	}
}

func anyToJSONRawMap(a any) map[string]jx.Raw { //todo: переписать этот ужас
	var m = map[string]jx.Raw{}
	if am, ok := a.(map[string]any); ok {
		for k, v := range am {
			m[k], _ = json.Marshal(v)
		}
		return m
	}
	t := reflect.ValueOf(a)
	switch t.Kind() {
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			b, err := json.Marshal(t.Field(i).Interface())
			if err != nil {
				panic("some shit")
			}
			m[t.Type().Field(i).Name] = b
		}
	default:
		panic(fmt.Sprintf("some shit %v", t.Kind()))
	}
	return m
}

func convertAccountAddress(id tongo.AccountID) oas.AccountAddress {
	return oas.AccountAddress{Address: id.ToRaw()}
}

func convertOptAccountAddress(id *tongo.AccountID) oas.OptAccountAddress {
	if id != nil {
		return oas.OptAccountAddress{Value: convertAccountAddress(*id), Set: true}
	}
	return oas.OptAccountAddress{}
}

func pointerToOptString(s *string) oas.OptString {
	var o oas.OptString
	if s != nil {
		o.SetTo(*s)
	}
	return o
}

func convertAccount(account *core.Account) oas.RawAccount {
	rawAccount := oas.RawAccount{
		Address:           account.AccountAddress.ToRaw(),
		Balance:           account.TonBalance,
		LastTransactionLt: account.LastTransactionLt,
		Status:            account.Status,
		Storage: oas.AccountStorageInfo{
			UsedCells:       account.Storage.UsedCells.Uint64(),
			UsedBits:        account.Storage.UsedBits.Uint64(),
			UsedPublicCells: account.Storage.UsedPublicCells.Uint64(),
			LastPaid:        int64(account.Storage.LastPaid),
			DuePayment:      account.Storage.DuePayment,
		},
	}
	if account.Code != nil && len(account.Code) != 0 {
		rawAccount.Code = oas.NewOptString(fmt.Sprintf("%x", account.Code[:]))
	}
	if account.Data != nil {
		rawAccount.Data = oas.NewOptString(fmt.Sprintf("%x", account.Data[:]))
	}
	return rawAccount
}
