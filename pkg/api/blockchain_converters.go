package api

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/tonkeeper/tongo/tlb"

	jsoniter "github.com/json-iterator/go"
	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
)

func blockIdFromString(s string) (tongo.BlockID, error) {
	var id tongo.BlockID
	_, err := fmt.Sscanf(s, "(%d,%x,%d)", &id.Workchain, &id.Shard, &id.Seqno)
	if err != nil {
		return tongo.BlockID{}, err
	}
	return id, nil
}

func blockIdExtFromString(s string) (tongo.BlockIDExt, error) {
	var (
		id                 tongo.BlockIDExt
		rootHash, fileHash []byte
	)
	_, err := fmt.Sscanf(s, "(%d,%x,%d,%x,%x)", &id.Workchain, &id.Shard, &id.Seqno, &rootHash, &fileHash)
	if err != nil {
		return tongo.BlockIDExt{}, err
	}
	err = id.RootHash.FromBytes(rootHash)
	if err != nil {
		return tongo.BlockIDExt{}, err
	}
	err = id.FileHash.FromBytes(fileHash)
	if err != nil {
		return tongo.BlockIDExt{}, err
	}
	return id, nil
}

func convertBlockHeader(b core.BlockHeader) oas.BlockchainBlock {
	res := oas.BlockchainBlock{
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

func convertTransaction(t core.Transaction, book addressBook, imgGenerator previewGenerator) oas.Transaction {
	tx := oas.Transaction{
		Hash:            t.Hash.Hex(),
		Lt:              int64(t.Lt),
		Account:         convertAccountAddress(t.Account, book, imgGenerator),
		Success:         t.Success,
		Utime:           t.Utime,
		OrigStatus:      oas.AccountStatus(t.OrigStatus),
		EndStatus:       oas.AccountStatus(t.EndStatus),
		TotalFees:       t.TotalFee,
		TransactionType: oas.TransactionType(t.Type),
		StateUpdateOld:  t.StateHashUpdate.OldHash.Hex(),
		StateUpdateNew:  t.StateHashUpdate.NewHash.Hex(),
		Block:           t.BlockID.String(),
		Aborted:         t.Aborted,
		Destroyed:       t.Destroyed,
	}
	if t.PrevTransLt != 0 {
		tx.PrevTransLt.Value = int64(t.PrevTransLt)
		tx.PrevTransLt.Set = true
		tx.PrevTransHash.Value = t.PrevTransHash.Hex()
		tx.PrevTransHash.Set = true
	}
	if t.InMsg != nil {
		tx.InMsg.SetTo(convertMessage(*t.InMsg, book, imgGenerator))
	}
	for _, m := range t.OutMsgs {
		tx.OutMsgs = append(tx.OutMsgs, convertMessage(m, book, imgGenerator))
	}
	if t.ActionPhase != nil {
		phase := oas.ActionPhase{
			Success:        t.ActionPhase.Success,
			TotalActions:   int32(t.ActionPhase.TotalActions),
			SkippedActions: int32(t.ActionPhase.SkippedActions),
			FwdFees:        int64(t.ActionPhase.FwdFees),
			TotalFees:      int64(t.ActionPhase.TotalFees),
		}
		tx.ActionPhase = oas.NewOptActionPhase(phase)
	}
	if t.StoragePhase != nil {
		phase := oas.StoragePhase{
			FeesCollected: int64(t.StoragePhase.StorageFeesCollected),
			StatusChange:  oas.AccStatusChange(t.StoragePhase.StatusChange),
		}
		if t.StoragePhase.StorageFeesDue != nil {
			phase.FeesDue = oas.NewOptInt64(int64(*t.StoragePhase.StorageFeesDue))
		}
		tx.StoragePhase = oas.NewOptStoragePhase(phase)
	}
	if t.ComputePhase != nil {
		phase := oas.ComputePhase{
			Skipped: t.ComputePhase.Skipped,
		}
		if t.ComputePhase.Skipped {
			phase.SkipReason = oas.NewOptComputeSkipReason(oas.ComputeSkipReason(t.ComputePhase.SkipReason))
		} else {
			phase.Success = oas.NewOptBool(t.ComputePhase.Success)
			phase.GasFees = oas.NewOptInt64(int64(t.ComputePhase.GasFees))
			phase.GasUsed = oas.NewOptInt64(t.ComputePhase.GasUsed.Int64())
			phase.VMSteps = oas.NewOptUint32(t.ComputePhase.VmSteps)
			phase.ExitCode = oas.NewOptInt32(t.ComputePhase.ExitCode)
		}
		tx.ComputePhase = oas.NewOptComputePhase(phase)
	}
	if t.CreditPhase != nil {
		phase := oas.CreditPhase{
			FeesCollected: int64(t.CreditPhase.DueFeesCollected),
			Credit:        int64(t.CreditPhase.CreditGrams),
		}
		tx.CreditPhase = oas.NewOptCreditPhase(phase)
	}
	if t.BouncePhase != nil {
		tx.BouncePhase = oas.NewOptBouncePhaseType(oas.BouncePhaseType(t.BouncePhase.Type))
	}
	return tx
}

func convertMessage(m core.Message, book addressBook, imgGenerator previewGenerator) oas.Message {
	msg := oas.Message{
		CreatedLt:   int64(m.CreatedLt),
		IhrDisabled: m.IhrDisabled,
		Bounce:      m.Bounce,
		Bounced:     m.Bounced,
		Value:       m.Value,
		FwdFee:      m.FwdFee,
		IhrFee:      m.IhrFee,
		Destination: convertOptAccountAddress(m.Destination, book, imgGenerator),
		Source:      convertOptAccountAddress(m.Source, book, imgGenerator),
		ImportFee:   m.ImportFee,
		CreatedAt:   int64(m.CreatedAt),
	}
	if len(m.Body) != 0 {
		msg.RawBody.SetTo(hex.EncodeToString(m.Body))
	}
	if m.OpCode != nil {
		msg.OpCode = oas.NewOptString("0x" + hex.EncodeToString(binary.BigEndian.AppendUint32(nil, *m.OpCode)))
	}
	if len(m.Init) != 0 {
		msg.Init.SetTo(oas.StateInit{
			Boc: hex.EncodeToString(m.Init),
		})
	}
	if m.DecodedBody != nil {
		msg.DecodedOpName = oas.NewOptString(g.CamelToSnake(m.DecodedBody.Operation))
		// DecodedBody.Value is a simple struct, there shouldn't be any issue with it.
		value, _ := jsoniter.Marshal(m.DecodedBody.Value)
		msg.DecodedBody = value
	}
	return msg
}

func convertConfig(cfg tlb.ConfigParams) (*oas.BlockchainConfig, error) {
	var config oas.BlockchainConfig
	for _, a := range []struct {
		index   tlb.Uint32
		pointer *string
	}{
		{0, &config.R0},
		{1, &config.R1},
		{2, &config.R2},
		{4, &config.R4},
	} {
		c, prs := cfg.Config.Get(a.index)
		if !prs {
			return nil, fmt.Errorf("config doesn't have %v param", a.index)
		}
		var b tlb.Bits256
		err := tlb.Unmarshal(&c.Value, &b)
		if err != nil {
			return nil, err
		}
		*a.pointer = b.Hex()
	}
	for _, a := range []struct {
		index   tlb.Uint32
		pointer *oas.OptValidatorsSet
	}{
		{32, &config.R32},
		{33, &config.R33},
		{34, &config.R34},
		{35, &config.R35},
		{36, &config.R36},
		{37, &config.R37},
	} {
		var set tlb.ValidatorsSet
		c, prs := cfg.Config.Get(a.index)
		if !prs {
			continue
		}
		err := tlb.Unmarshal(&c.Value, &set)
		if err != nil {
			return nil, err
		}
		*a.pointer = convertValidatorSet(set)
	}
	{
		c, prs := cfg.Config.Get(44)
		if !prs {
			return nil, fmt.Errorf("config doesn't have %v param", 44)
		}
		var blockedAccounts struct {
			Prefix         byte
			Map            tlb.HashmapE[accID, struct{}]
			SuspendedUntil uint32
		}
		err := tlb.Unmarshal(&c.Value, &blockedAccounts)
		if err != nil {
			return nil, err
		}
		config.R44.SetSuspendedUntil(int(blockedAccounts.SuspendedUntil))
		for _, a := range blockedAccounts.Map.Items() {
			config.R44.Accounts = append(config.R44.Accounts, tongo.AccountID(a.Key).String())
		}
	}

	return &config, nil
}

func convertValidatorSet(set tlb.ValidatorsSet) oas.OptValidatorsSet {
	var s oas.ValidatorsSet
	s.UtimeUntil = int(set.Common().UtimeUntil)
	s.UtimeSince = int(set.Common().UtimeSince)
	s.Main = int(set.Common().Main)
	s.Total = int(set.Common().Total)
	var l []tlb.ValidatorDescr
	if set.SumType == "Validators" {
		l = set.Validators.List.Values()
	} else {
		l = set.ValidatorsExt.List.Values()
	}
	for _, d := range l {
		s.List = append(s.List, oas.ValidatorsSetListItem{PublicKey: d.PubKey().Hex()})
	}
	return oas.NewOptValidatorsSet(s)
}

type accID tongo.AccountID

func (a accID) FixedSize() int {
	return 288 // (32+256) * 8
}
