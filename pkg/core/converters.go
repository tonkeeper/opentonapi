package core

import (
	"fmt"
	"math/big"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
)

func ConvertToBlockHeader(id tongo.BlockIDExt, block *tlb.Block) (*BlockHeader, error) {
	info := block.Info
	header := &BlockHeader{
		BlockIDExt: id,
		// todo: why do we have all these type conversions?
		StartLt:                int64(info.StartLt),
		EndLt:                  int64(info.EndLt),
		GlobalId:               block.GlobalId,
		MinRefMcSeqno:          int32(info.MinRefMcSeqno),
		CatchainSeqno:          int32(info.GenCatchainSeqno),
		PrevKeyBlockSeqno:      int32(info.PrevKeyBlockSeqno),
		ValidatorListHashShort: int32(info.GenValidatorListHashShort),
		Version:                info.Version,
		VertSeqno:              info.VertSeqNo,
		GenUtime:               info.GenUtime,
		WantMerge:              info.WantMerge,
		WantSplit:              info.WantSplit,
		AfterMerge:             info.AfterMerge,
		AfterSplit:             info.AfterSplit,
		BeforeSplit:            info.BeforeSplit,
		IsKeyBlock:             info.KeyBlock,
		BlockExtra: BlockExtra{
			RandSeed:          tongo.Bits256(block.Extra.RandSeed),
			CreatedBy:         tongo.Bits256(block.Extra.CreatedBy),
			InMsgDescrLength:  len(block.Extra.InMsgDescr.Keys()),
			OutMsgDescrLength: len(block.Extra.OutMsgDescr.Keys()),
		},
	}
	if info.GenSoftware != nil {
		header.GenSoftware = &GenSoftware{
			Version:      info.GenSoftware.Version,
			Capabilities: info.GenSoftware.Capabilities,
		}
	}
	parents, err := tongo.GetParents(info)
	if err != nil {
		return nil, err
	}
	for _, parent := range parents {
		header.PrevBlocks = append(header.PrevBlocks, tongo.BlockIDExt(parent))
	}
	if info.MasterRef != nil {
		header.MasterRef = &tongo.BlockIDExt{
			BlockID: tongo.BlockID{
				Workchain: -1,
				Seqno:     info.MasterRef.Master.SeqNo,
			},
			RootHash: tongo.Bits256(info.MasterRef.Master.RootHash),
			FileHash: tongo.Bits256(info.MasterRef.Master.FileHash),
		}
	}
	return header, nil
}

func convertComputePhase(phase tlb.TrComputePhase) *TxComputePhase {
	switch phase.SumType {
	case "TrPhaseComputeSkipped":
		return &TxComputePhase{
			Skipped:    true,
			SkipReason: phase.TrPhaseComputeSkipped.Reason,
		}
	default:
		return &TxComputePhase{
			Success:  phase.TrPhaseComputeVm.Success,
			GasFees:  uint64(phase.TrPhaseComputeVm.GasFees),
			GasUsed:  big.Int(phase.TrPhaseComputeVm.Vm.GasUsed),
			VmSteps:  phase.TrPhaseComputeVm.Vm.VmSteps,
			ExitCode: phase.TrPhaseComputeVm.Vm.ExitCode,
		}
	}
}

func convertCreditPhase(phase tlb.TrCreditPhase) *TxCreditPhase {
	ph := TxCreditPhase{
		CreditGrams: uint64(phase.Credit.Grams),
	}
	if phase.DueFeesCollected.Exists {
		ph.DueFeesCollected = uint64(phase.DueFeesCollected.Value)
	}
	return &ph
}

func convertStoragePhase(phase tlb.TrStoragePhase) *TxStoragePhase {
	ph := TxStoragePhase{
		StorageFeesCollected: uint64(phase.StorageFeesCollected),
		StatusChange:         phase.StatusChange,
	}
	if phase.StorageFeesDue.Exists {
		value := uint64(phase.StorageFeesDue.Value)
		ph.StorageFeesDue = &value
	}
	return &ph
}

func convertActionPhase(phase tlb.TrActionPhase) *TxActionPhase {
	return &TxActionPhase{
		Success:        phase.Success,
		TotalActions:   phase.TotActions,
		SkippedActions: phase.SkippedActions,
		FwdFees:        uint64(phase.TotalFwdFees.Value),
		TotalFees:      uint64(phase.TotalActionFees.Value),
	}
}
func convertBouncePhase(phase tlb.TrBouncePhase) *TxBouncePhase {
	return &TxBouncePhase{
		Type: BouncePhaseType(phase.SumType),
	}
}

func maybeInvoke[Result any, T any](fn func(t T) *Result, maybe tlb.Maybe[T]) *Result {
	if !maybe.Exists {
		return nil
	}
	return fn(maybe.Value)
}

func maybeRefInvoke[Result any, T any](fn func(t T) *Result, maybe tlb.Maybe[tlb.Ref[T]]) *Result {
	if !maybe.Exists {
		return nil
	}
	return fn(maybe.Value.Value)
}

func ConvertTransaction(workchain int32, tx tongo.Transaction) (*Transaction, error) {
	var (
		totalFee int64
		inMsg    *Message
	)
	if tx.Msgs.InMsg.Exists {
		msg, err := ConvertMessage(tx.Msgs.InMsg.Value.Value, tx.Lt)
		if err != nil {
			return nil, err
		}
		inMsg = &msg
		if msg.Source == nil {
			totalFee += msg.FwdFee
		}
	}
	var outMessage []Message
	for _, msg := range tx.Msgs.OutMsgs.Values() {
		m, err := ConvertMessage(msg.Value, tx.Lt)
		if err != nil {
			return nil, err
		}
		outMessage = append(outMessage, m)
		totalFee += m.FwdFee
	}
	var computePhase *TxComputePhase
	var storagePhase *TxStoragePhase
	var creditPhase *TxCreditPhase
	var actionPhase *TxActionPhase
	var bouncePhase *TxBouncePhase
	var aborted bool
	var destroyed bool

	desc := tx.Description
	switch desc.SumType {
	case "TransOrd":
		tx := desc.TransOrd
		aborted = tx.Aborted
		destroyed = tx.Destroyed
		computePhase = convertComputePhase(tx.ComputePh)
		storagePhase = maybeInvoke(convertStoragePhase, tx.StoragePh)
		creditPhase = maybeInvoke(convertCreditPhase, tx.CreditPh)
		bouncePhase = maybeInvoke(convertBouncePhase, tx.Bounce)
		actionPhase = maybeRefInvoke(convertActionPhase, tx.Action)
	case "TransStorage":
		tx := desc.TransStorage
		storagePhase = convertStoragePhase(tx.StoragePh)
	case "TransTickTock":
		tx := desc.TransTickTock
		aborted = tx.Aborted
		destroyed = tx.Destroyed
		computePhase = convertComputePhase(tx.ComputePh)
		storagePhase = convertStoragePhase(tx.StoragePh)
		actionPhase = maybeRefInvoke(convertActionPhase, tx.Action)
	case "TransSplitPrepare":
		tx := desc.TransSplitPrepare
		aborted = tx.Aborted
		destroyed = tx.Destroyed
		computePhase = convertComputePhase(tx.ComputePh)
		storagePhase = maybeInvoke(convertStoragePhase, tx.StoragePh)
		actionPhase = maybeRefInvoke(convertActionPhase, tx.Action)
	case "TransMergePrepare":
		tx := desc.TransMergePrepare
		aborted = tx.Aborted
		storagePhase = convertStoragePhase(tx.StoragePh)
	case "TransMergeInstall":
		tx := desc.TransMergeInstall
		aborted = tx.Aborted
		destroyed = tx.Destroyed
		computePhase = convertComputePhase(tx.ComputePh)
		storagePhase = maybeInvoke(convertStoragePhase, tx.StoragePh)
		creditPhase = maybeInvoke(convertCreditPhase, tx.CreditPh)
		actionPhase = maybeRefInvoke(convertActionPhase, tx.Action)
	}
	totalFee += int64(tx.TotalFees.Grams)

	return &Transaction{
		TransactionID: TransactionID{
			Hash:    tongo.Bits256(tx.Hash()),
			Lt:      tx.Lt,
			Account: *tongo.NewAccountId(workchain, tx.AccountAddr),
		},
		StateHashUpdate: tx.StateUpdate,
		Type:            TransactionType(desc.SumType),
		Data:            []byte{},
		Fee:             totalFee,
		BlockID:         tx.BlockID.BlockID,
		//StorageFee:    storageFee,
		Utime:         int64(tx.Now),
		InMsg:         inMsg,
		OutMsgs:       outMessage,
		PrevTransHash: tongo.Bits256(tx.PrevTransHash),
		PrevTransLt:   tx.PrevTransLt,
		Success:       tx.IsSuccess(),
		OrigStatus:    tx.OrigStatus,
		EndStatus:     tx.EndStatus,
		Aborted:       aborted,
		Destroyed:     destroyed,
		ComputePhase:  computePhase,
		StoragePhase:  storagePhase,
		CreditPhase:   creditPhase,
		ActionPhase:   actionPhase,
		BouncePhase:   bouncePhase,
	}, nil
}

func ConvertMessage(message tlb.Message, txLT uint64) (Message, error) {
	init := make([]byte, 0)
	if message.Init.Exists {
		initCell := boc.NewCell()
		err := tlb.Marshal(initCell, message.Init.Value.Value)
		if err != nil {
			return Message{}, err
		}
		init, err = initCell.ToBoc()
		if err != nil {
			return Message{}, err
		}
	}
	cell := boc.Cell(message.Body.Value)
	var decodedBody *DecodedMessageBody
	if op, value, err := abi.MessageDecoder(&cell); err == nil {
		decodedBody = &DecodedMessageBody{
			Operation: op,
			Value:     value,
		}
	}
	cell.ResetCounters()
	var opCode *uint32
	if code, err := cell.PickUint(32); err == nil {
		opCode = g.Pointer(uint32(code))
	}
	body, err := convertBodyCell(message.Body.Value)
	if err != nil {
		return Message{}, err
	}
	switch message.Info.SumType {
	case "IntMsgInfo":
		info := message.Info.IntMsgInfo
		source, err := tongo.AccountIDFromTlb(info.Src)
		if err != nil {
			return Message{}, err
		}
		dest, err := tongo.AccountIDFromTlb(info.Dest)
		if err != nil {
			return Message{}, err
		}
		return Message{
			MessageID: MessageID{
				CreatedLt:   info.CreatedLt,
				Source:      source,
				Destination: dest,
			},
			IhrDisabled: info.IhrDisabled,
			Bounce:      info.Bounce,
			Bounced:     info.Bounced,
			Value:       int64(info.Value.Grams),
			FwdFee:      int64(info.FwdFee),
			IhrFee:      int64(info.IhrFee),
			ImportFee:   0,
			Init:        init,
			Body:        body,
			OpCode:      opCode,
			DecodedBody: decodedBody,
			CreatedAt:   info.CreatedAt,
		}, nil
	case "ExtInMsgInfo":
		info := message.Info.ExtInMsgInfo
		dest, err := tongo.AccountIDFromTlb(info.Dest)
		if err != nil {
			return Message{}, err
		}
		return Message{
			MessageID: MessageID{
				CreatedLt:   txLT,
				Destination: dest,
			},
			ImportFee:   int64(info.ImportFee),
			Body:        body,
			OpCode:      opCode,
			DecodedBody: decodedBody,
			Init:        init,
		}, nil
	case "ExtOutMsgInfo":
		info := message.Info.ExtOutMsgInfo
		source, err := tongo.AccountIDFromTlb(info.Src)
		if err != nil {
			return Message{}, err
		}
		return Message{
			MessageID: MessageID{
				CreatedLt: info.CreatedLt,
				Source:    source,
			},
			Body:        body,
			DecodedBody: decodedBody,
			CreatedAt:   info.CreatedAt,
			OpCode:      opCode,
			Init:        []byte{},
		}, nil
	}
	return Message{}, fmt.Errorf("invalid message")
}

func convertBodyCell(a tlb.Any) ([]byte, error) {
	c := boc.Cell(a)
	if c.BitsAvailableForRead() == 0 {
		return []byte{}, nil
	}
	if c.BitsAvailableForRead() >= 32 {
		v, _ := c.PickUint(32)
		if v == 0 && c.BitsAvailableForRead()%8 == 0 && c.RefsSize() == 0 { // Text payload
			bs := c.ReadRemainingBits()
			b, err := bs.GetTopUppedArray()
			if err != nil {
				return nil, err
			}
			return b, nil
		}
	}
	return c.ToBoc()
}
