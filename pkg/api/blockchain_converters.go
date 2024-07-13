package api

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"

	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
)

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

func convertValueFlow(collection core.CurrencyCollection) oas.BlockCurrencyCollection {
	res := oas.BlockCurrencyCollection{
		Grams: int64(collection.Grams),
		Other: make([]oas.BlockCurrencyCollectionOtherItem, 0, len(collection.Other)),
	}
	for _, c := range collection.Other {
		res.Other = append(res.Other, oas.BlockCurrencyCollectionOtherItem{
			ID:    c.ID,
			Value: c.Value,
		})
	}
	sort.Slice(res.Other, func(i, j int) bool {
		return res.Other[i].ID < res.Other[j].ID
	})
	return res
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
		TxQuantity:        b.TxQuantity,
		ValueFlow: oas.BlockValueFlow{
			FromPrevBlk:   convertValueFlow(b.ValueFlow.FromPrevBlk),
			ToNextBlk:     convertValueFlow(b.ValueFlow.ToNextBlk),
			Imported:      convertValueFlow(b.ValueFlow.Imported),
			Exported:      convertValueFlow(b.ValueFlow.Exported),
			FeesCollected: convertValueFlow(b.ValueFlow.FeesCollected),
			FeesImported:  convertValueFlow(b.ValueFlow.FeesImported),
			Recovered:     convertValueFlow(b.ValueFlow.Recovered),
			Created:       convertValueFlow(b.ValueFlow.Created),
			Minted:        convertValueFlow(b.ValueFlow.Minted),
		},
	}
	if b.ValueFlow.Burned != nil {
		res.ValueFlow.Burned = oas.NewOptBlockCurrencyCollection(convertValueFlow(*b.ValueFlow.Burned))
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

func convertReducedBlock(block core.ReducedBlock) oas.ReducedBlock {
	converted := oas.ReducedBlock{
		WorkchainID: block.Workchain,
		Shard:       fmt.Sprintf("%016x", block.Shard),
		Seqno:       int32(block.Seqno),
		Utime:       block.Utime,
		TxQuantity:  block.TxQuantity,
	}
	if block.MasterRef != nil {
		converted.MasterRef.SetTo(block.MasterRef.String())
	}
	for _, s := range block.ShardsBlocks {
		converted.ShardsBlocks = append(converted.ShardsBlocks, s.String())
	}
	for _, s := range block.ParentBlocks {
		converted.Parent = append(converted.Parent, s.String())
	}
	return converted
}

func convertTransaction(t core.Transaction, accountInterfaces []abi.ContractInterface, book addressBook) oas.Transaction {
	tx := oas.Transaction{
		Hash:            t.Hash.Hex(),
		Lt:              int64(t.Lt),
		Account:         convertAccountAddress(t.Account, book),
		Success:         t.Success,
		Utime:           t.Utime,
		OrigStatus:      oas.AccountStatus(t.OrigStatus),
		EndStatus:       oas.AccountStatus(t.EndStatus),
		TotalFees:       t.TotalFee,
		EndBalance:      t.EndBalance,
		TransactionType: oas.TransactionType(t.Type),
		StateUpdateOld:  t.StateHashUpdate.OldHash.Hex(),
		StateUpdateNew:  t.StateHashUpdate.NewHash.Hex(),
		Block:           t.BlockID.String(),
		Aborted:         t.Aborted,
		Destroyed:       t.Destroyed,
		Raw:             hex.EncodeToString(t.Raw),
	}
	if t.PrevTransLt != 0 {
		tx.PrevTransLt.Value = int64(t.PrevTransLt)
		tx.PrevTransLt.Set = true
		tx.PrevTransHash.Value = t.PrevTransHash.Hex()
		tx.PrevTransHash.Set = true
	}
	if t.InMsg != nil {
		tx.InMsg.SetTo(convertMessage(*t.InMsg, book))
	}
	for _, m := range t.OutMsgs {
		tx.OutMsgs = append(tx.OutMsgs, convertMessage(m, book))
	}
	sort.Slice(tx.OutMsgs, func(i, j int) bool {
		return tx.OutMsgs[i].CreatedLt < tx.OutMsgs[j].CreatedLt
	})
	if t.ActionPhase != nil {
		phase := oas.ActionPhase{
			Success:               t.ActionPhase.Success,
			ResultCode:            t.ActionPhase.ResultCode,
			TotalActions:          int32(t.ActionPhase.TotalActions),
			SkippedActions:        int32(t.ActionPhase.SkippedActions),
			FwdFees:               int64(t.ActionPhase.FwdFees),
			TotalFees:             int64(t.ActionPhase.TotalFees),
			ResultCodeDescription: g.Opt(convertActionPhaseResultCode(t.ActionPhase.ResultCode)),
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
			phase.VMSteps = oas.NewOptInt32(int32(t.ComputePhase.VmSteps))
			phase.ExitCode = oas.NewOptInt32(t.ComputePhase.ExitCode)
			phase.ExitCodeDescription = g.Opt(abi.GetContractError(accountInterfaces, t.ComputePhase.ExitCode))
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

func convertMsgType(msgType core.MsgType) oas.MessageMsgType {
	switch msgType {
	case core.ExtInMsg:
		return oas.MessageMsgTypeExtInMsg
	case core.ExtOutMsg:
		return oas.MessageMsgTypeExtOutMsg
	default:
		return oas.MessageMsgTypeIntMsg
	}
}

func convertMessage(m core.Message, book addressBook) oas.Message {
	msg := oas.Message{
		MsgType:       convertMsgType(m.MsgType),
		CreatedLt:     int64(m.CreatedLt),
		IhrDisabled:   m.IhrDisabled,
		Bounce:        m.Bounce,
		Bounced:       m.Bounced,
		Value:         m.Value,
		FwdFee:        m.FwdFee,
		IhrFee:        m.IhrFee,
		Destination:   convertOptAccountAddress(m.Destination, book),
		Source:        convertOptAccountAddress(m.Source, book),
		ImportFee:     m.ImportFee,
		CreatedAt:     int64(m.CreatedAt),
		OpCode:        oas.OptString{},
		Init:          oas.OptStateInit{},
		RawBody:       oas.OptString{},
		DecodedOpName: oas.OptString{},
		DecodedBody:   nil,
		Hash:          m.Hash.Hex(),
	}
	if len(m.Body) != 0 {
		msg.RawBody.SetTo(hex.EncodeToString(m.Body))
	}
	if m.OpCode != nil {
		msg.OpCode = oas.NewOptString("0x" + hex.EncodeToString(binary.BigEndian.AppendUint32(nil, *m.OpCode)))
	}
	if len(m.Init) != 0 {
		interfaces := make([]string, len(m.InitInterfaces))
		for i, iface := range m.InitInterfaces {
			interfaces[i] = iface.String()
		}
		msg.Init.SetTo(oas.StateInit{
			Boc:        hex.EncodeToString(m.Init),
			Interfaces: interfaces,
		})
	}
	if m.DecodedBody != nil {
		msg.DecodedOpName = oas.NewOptString(g.CamelToSnake(m.DecodedBody.Operation))
		// DecodedBody.Value is a simple struct, there shouldn't be any issue with it.
		value, _ := json.Marshal(m.DecodedBody.Value)
		msg.DecodedBody = g.ChangeJsonKeys(value, g.CamelToSnake)
	}
	return msg
}

func convertConfig(logger *zap.Logger, cfg tlb.ConfigParams) (*oas.BlockchainConfig, error) {
	// TODO: configParam39
	var config oas.BlockchainConfig
	blockchainConfig, err := ton.ConvertBlockchainConfigStrict(cfg)
	if err != nil {
		return nil, err
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
	if addr, ok := blockchainConfig.ConfigAddr(); ok {
		config.R0 = addr.ToRaw()
	}
	if addr, ok := blockchainConfig.ElectorAddr(); ok {
		config.R1 = addr.ToRaw()
	}
	if addr, ok := blockchainConfig.MinterAddr(); ok {
		config.R2 = addr.ToRaw()
	}
	if addr, ok := blockchainConfig.FeeCollectorAddr(); ok {
		config.R3 = oas.NewOptString(addr.ToRaw())
	}
	if addr, ok := blockchainConfig.DnsRootAddr(); ok {
		config.R4 = addr.ToRaw()
	}
	if p5 := blockchainConfig.ConfigParam5; p5 != nil {
		param5 := oas.BlockchainConfig5{
			FeeBurnNom:   int64(p5.BurningConfig.FeeBurnNom),
			FeeBurnDenom: int64(p5.BurningConfig.FeeBurnDenom),
		}
		if p5.BurningConfig.BlackholeAddr != nil {
			accountID := ton.AccountID{Workchain: -1, Address: *p5.BurningConfig.BlackholeAddr}
			param5.BlackholeAddr = oas.NewOptString(accountID.ToRaw())
		}
		config.R5 = oas.NewOptBlockchainConfig5(param5)
	}
	if p6 := blockchainConfig.ConfigParam6; p6 != nil {
		param6 := oas.BlockchainConfig6{
			MintNewPrice: int64(p6.MintNewPrice),
			MintAddPrice: int64(p6.MintAddPrice),
		}
		config.R6 = oas.NewOptBlockchainConfig6(param6)
	}
	if p7 := blockchainConfig.ConfigParam7; p7 != nil {
		param7 := oas.BlockchainConfig7{
			Currencies: make([]oas.BlockchainConfig7CurrenciesItem, 0, len(p7.ToMint.Dict.Items())),
		}
		for _, item := range p7.ToMint.Dict.Items() {
			value := big.Int(item.Value)
			param7.Currencies = append(param7.Currencies, oas.BlockchainConfig7CurrenciesItem{
				CurrencyID: int64(item.Key),
				Amount:     value.String(),
			})
		}
		config.R7 = oas.NewOptBlockchainConfig7(param7)
	}
	if p8 := blockchainConfig.ConfigParam8; p8 != nil {
		param8 := oas.BlockchainConfig8{
			Version:      int64(p8.GlobalVersion.Version),
			Capabilities: int64(p8.GlobalVersion.Capabilities),
		}
		config.R8 = oas.NewOptBlockchainConfig8(param8)
	}
	if p9 := blockchainConfig.ConfigParam9; p9 != nil {
		param9 := oas.BlockchainConfig9{
			MandatoryParams: make([]int32, 0, len(p9.MandatoryParams.Keys())),
		}
		for _, param := range p9.MandatoryParams.Keys() {
			param9.MandatoryParams = append(param9.MandatoryParams, int32(param))
		}
		config.R9 = oas.NewOptBlockchainConfig9(param9)
	}
	if p10 := blockchainConfig.ConfigParam10; p10 != nil {
		param10 := oas.BlockchainConfig10{
			CriticalParams: make([]int32, 0, len(p10.CriticalParams.Keys())),
		}
		for _, param := range p10.CriticalParams.Keys() {
			param10.CriticalParams = append(param10.CriticalParams, int32(param))
		}
		config.R10 = oas.NewOptBlockchainConfig10(param10)
	}
	if p11 := blockchainConfig.ConfigParam11; p11 != nil {
		param11 := oas.BlockchainConfig11{
			NormalParams: oas.ConfigProposalSetup{
				MinTotRounds: int(p11.ConfigVotingSetup.NormalParams.MinTotRounds),
				MaxTotRounds: int(p11.ConfigVotingSetup.NormalParams.MaxTotRounds),
				MinWins:      int(p11.ConfigVotingSetup.NormalParams.MinWins),
				MaxLosses:    int(p11.ConfigVotingSetup.NormalParams.MaxLosses),
				MinStoreSec:  int64(p11.ConfigVotingSetup.NormalParams.MinStoreSec),
				MaxStoreSec:  int64(p11.ConfigVotingSetup.NormalParams.MaxStoreSec),
				BitPrice:     int64(p11.ConfigVotingSetup.NormalParams.BitPrice),
				CellPrice:    int64(p11.ConfigVotingSetup.NormalParams.CellPrice),
			},
			CriticalParams: oas.ConfigProposalSetup{
				MinTotRounds: int(p11.ConfigVotingSetup.CriticalParams.MinTotRounds),
				MaxTotRounds: int(p11.ConfigVotingSetup.CriticalParams.MaxTotRounds),
				MinWins:      int(p11.ConfigVotingSetup.CriticalParams.MinWins),
				MaxLosses:    int(p11.ConfigVotingSetup.CriticalParams.MaxLosses),
				MinStoreSec:  int64(p11.ConfigVotingSetup.CriticalParams.MinStoreSec),
				MaxStoreSec:  int64(p11.ConfigVotingSetup.CriticalParams.MaxStoreSec),
				BitPrice:     int64(p11.ConfigVotingSetup.CriticalParams.BitPrice),
				CellPrice:    int64(p11.ConfigVotingSetup.CriticalParams.CellPrice),
			},
		}
		config.R11 = oas.NewOptBlockchainConfig11(param11)
	}
	if p12 := blockchainConfig.ConfigParam12; p12 != nil {
		workchains := make([]oas.WorkchainDescr, 0, len(p12.Workchains.Keys()))
		for _, item := range p12.Workchains.Items() {
			workchains = append(workchains, convertWorkchain(logger, item.Key, item.Value))
		}
		param12 := oas.BlockchainConfig12{
			Workchains: workchains,
		}
		config.R12 = oas.NewOptBlockchainConfig12(param12)
	}
	if p13 := blockchainConfig.ConfigParam13; p13 != nil {
		param13 := oas.BlockchainConfig13{
			Deposit:   int64(p13.ComplaintPricing.Deposit),
			BitPrice:  int64(p13.ComplaintPricing.BitPrice),
			CellPrice: int64(p13.ComplaintPricing.CellPrice),
		}
		config.R13 = oas.NewOptBlockchainConfig13(param13)
	}
	if p14 := blockchainConfig.ConfigParam14; p14 != nil {
		param14 := oas.BlockchainConfig14{
			MasterchainBlockFee: int64(p14.BlockCreateFees.MasterchainBlockFee),
			BasechainBlockFee:   int64(p14.BlockCreateFees.BasechainBlockFee),
		}
		config.R14 = oas.NewOptBlockchainConfig14(param14)
	}
	if p15 := blockchainConfig.ConfigParam15; p15 != nil {
		param15 := oas.BlockchainConfig15{
			ValidatorsElectedFor: int64(p15.ValidatorsElectedFor),
			ElectionsStartBefore: int64(p15.ElectionsStartBefore),
			ElectionsEndBefore:   int64(p15.ElectionsEndBefore),
			StakeHeldFor:         int64(p15.StakeHeldFor),
		}
		config.R15 = oas.NewOptBlockchainConfig15(param15)
	}
	if p16 := blockchainConfig.ConfigParam16; p16 != nil {
		param16 := oas.BlockchainConfig16{
			MaxValidators:     int(p16.MaxValidators),
			MinValidators:     int(p16.MinValidators),
			MaxMainValidators: int(p16.MaxMainValidators),
		}
		config.R16 = oas.NewOptBlockchainConfig16(param16)
	}
	if p17 := blockchainConfig.ConfigParam17; p17 != nil {
		param17 := oas.BlockchainConfig17{
			MinStake:       fmt.Sprintf("%d", p17.MinStake),
			MaxStake:       fmt.Sprintf("%d", p17.MaxStake),
			MaxStakeFactor: int64(p17.MaxStakeFactor),
			MinTotalStake:  fmt.Sprintf("%d", p17.MinTotalStake),
		}
		config.R17 = oas.NewOptBlockchainConfig17(param17)
	}
	if p18 := blockchainConfig.ConfigParam18; p18 != nil {
		param18 := oas.BlockchainConfig18{
			StoragePrices: make([]oas.BlockchainConfig18StoragePricesItem, 0, len(p18.Value.Keys())),
		}
		for _, item := range p18.Value.Values() {
			param18.StoragePrices = append(param18.StoragePrices, oas.BlockchainConfig18StoragePricesItem{
				UtimeSince:    int64(item.UtimeSince),
				BitPricePs:    int64(item.BitPricePs),
				CellPricePs:   int64(item.CellPricePs),
				McBitPricePs:  int64(item.McBitPricePs),
				McCellPricePs: int64(item.McCellPricePs),
			})
		}
		config.R18 = oas.NewOptBlockchainConfig18(param18)
	}
	if p20 := blockchainConfig.ConfigParam20; p20 != nil {
		param20 := oas.BlockchainConfig20{
			GasLimitsPrices: convertGasLimitsPrices(logger, p20.GasLimitsPrices),
		}
		config.R20 = oas.NewOptBlockchainConfig20(param20)
	}
	if p21 := blockchainConfig.ConfigParam21; p21 != nil {
		param21 := oas.BlockchainConfig21{
			GasLimitsPrices: convertGasLimitsPrices(logger, p21.GasLimitsPrices),
		}
		config.R21 = oas.NewOptBlockchainConfig21(param21)
	}
	if p22 := blockchainConfig.ConfigParam22; p22 != nil {
		param22 := oas.BlockchainConfig22{
			BlockLimits: convertBlockLimits(p22.BlockLimits),
		}
		config.R22 = oas.NewOptBlockchainConfig22(param22)
	}
	if p23 := blockchainConfig.ConfigParam23; p23 != nil {
		param23 := oas.BlockchainConfig23{
			BlockLimits: convertBlockLimits(p23.BlockLimits),
		}
		config.R23 = oas.NewOptBlockchainConfig23(param23)
	}
	if p24 := blockchainConfig.ConfigParam24; p24 != nil {
		param24 := oas.BlockchainConfig24{
			MsgForwardPrices: convertMsgForwardPrices(p24.MsgForwardPrices),
		}
		config.R24 = oas.NewOptBlockchainConfig24(param24)
	}
	if p25 := blockchainConfig.ConfigParam25; p25 != nil {
		param25 := oas.BlockchainConfig25{
			MsgForwardPrices: convertMsgForwardPrices(p25.MsgForwardPrices),
		}
		config.R25 = oas.NewOptBlockchainConfig25(param25)
	}
	if p28 := blockchainConfig.ConfigParam28; p28 != nil {
		config.R28 = oas.NewOptBlockchainConfig28(convertCatchainConfig(logger, p28.CatchainConfig))
	}
	if p29 := blockchainConfig.ConfigParam29; p29 != nil {
		config.R29 = oas.NewOptBlockchainConfig29(convertConsensusConfig(logger, p29.ConsensusConfig))
	}
	if p31 := blockchainConfig.ConfigParam31; p31 != nil {
		param31 := oas.BlockchainConfig31{
			FundamentalSmcAddr: make([]string, 0, len(p31.FundamentalSmcAddr.Keys())),
		}
		for _, addr := range p31.FundamentalSmcAddr.Keys() {
			accountID := ton.AccountID{Workchain: -1, Address: addr}
			param31.FundamentalSmcAddr = append(param31.FundamentalSmcAddr, accountID.ToRaw())
		}
		config.R31 = oas.NewOptBlockchainConfig31(param31)
	}
	if p43 := blockchainConfig.ConfigParam43; p43 != nil {
		param43 := oas.BlockchainConfig43{
			SizeLimitsConfig: convertSizeLimitsConfig(logger, p43.SizeLimitsConfig),
		}
		config.R43 = oas.NewOptBlockchainConfig43(param43)
	}
	if blockchainConfig.ConfigParam44 == nil {
		return nil, fmt.Errorf("config doesn't have %v param", 44)
	}
	for _, addr := range blockchainConfig.ConfigParam44.SuspendedAddressList.Addresses.Keys() {
		accountID := ton.AccountID{
			Workchain: int32(addr.Workchain),
			Address:   addr.Address,
		}
		config.R44.Accounts = append(config.R44.Accounts, accountID.String())
	}
	config.R44.SetSuspendedUntil(int(blockchainConfig.ConfigParam44.SuspendedAddressList.SuspendedUntil))
	if p71 := blockchainConfig.ConfigParam71; p71 != nil {
		param71 := oas.BlockchainConfig71{
			OracleBridgeParams: convertOracleBridgeParams(p71.OracleBridgeParams),
		}
		config.R71 = oas.NewOptBlockchainConfig71(param71)
	}
	if p72 := blockchainConfig.ConfigParam72; p72 != nil {
		param72 := oas.BlockchainConfig72{
			OracleBridgeParams: convertOracleBridgeParams(p72.OracleBridgeParams),
		}
		config.R72 = oas.NewOptBlockchainConfig72(param72)
	}
	if p73 := blockchainConfig.ConfigParam73; p73 != nil {
		param73 := oas.BlockchainConfig73{
			OracleBridgeParams: convertOracleBridgeParams(p73.OracleBridgeParams),
		}
		config.R73 = oas.NewOptBlockchainConfig73(param73)
	}
	if p79 := blockchainConfig.ConfigParam79; p79 != nil {
		param79 := oas.BlockchainConfig79{
			JettonBridgeParams: convertJettonBridgeParams(logger, p79.JettonBridgeParams),
		}
		config.R79 = oas.NewOptBlockchainConfig79(param79)
	}
	if p81 := blockchainConfig.ConfigParam81; p81 != nil {
		param81 := oas.BlockchainConfig81{
			JettonBridgeParams: convertJettonBridgeParams(logger, p81.JettonBridgeParams),
		}
		config.R81 = oas.NewOptBlockchainConfig81(param81)
	}
	if p82 := blockchainConfig.ConfigParam82; p82 != nil {
		param82 := oas.BlockchainConfig82{
			JettonBridgeParams: convertJettonBridgeParams(logger, p82.JettonBridgeParams),
		}
		config.R82 = oas.NewOptBlockchainConfig82(param82)
	}
	return &config, nil
}

func convertWorkchain(logger *zap.Logger, workchain tlb.Uint32, desc tlb.WorkchainDescr) oas.WorkchainDescr {
	switch desc.SumType {
	case "Workchain":
		return oas.WorkchainDescr{
			Workchain:         int(workchain),
			EnabledSince:      int64(desc.Workchain.EnabledSince),
			ActualMinSplit:    int(desc.Workchain.ActualMinSplit),
			MinSplit:          int(desc.Workchain.MinSplit),
			MaxSplit:          int(desc.Workchain.MaxSplit),
			Basic:             int(desc.Workchain.Basic),
			Active:            desc.Workchain.Active,
			AcceptMsgs:        desc.Workchain.AcceptMsgs,
			Flags:             int(desc.Workchain.Flags),
			ZerostateRootHash: desc.Workchain.ZerostateRootHash.Hex(),
			ZerostateFileHash: desc.Workchain.ZerostateFileHash.Hex(),
			Version:           int64(desc.Workchain.Version),
		}
	case "WorkchainV2":
		return oas.WorkchainDescr{
			Workchain:         int(workchain),
			EnabledSince:      int64(desc.WorkchainV2.EnabledSince),
			ActualMinSplit:    int(desc.WorkchainV2.ActualMinSplit),
			MinSplit:          int(desc.WorkchainV2.MinSplit),
			MaxSplit:          int(desc.WorkchainV2.MaxSplit),
			Basic:             int(desc.WorkchainV2.Basic),
			Active:            desc.WorkchainV2.Active,
			AcceptMsgs:        desc.WorkchainV2.AcceptMsgs,
			Flags:             int(desc.WorkchainV2.Flags),
			ZerostateRootHash: desc.WorkchainV2.ZerostateRootHash.Hex(),
			ZerostateFileHash: desc.WorkchainV2.ZerostateFileHash.Hex(),
			Version:           int64(desc.WorkchainV2.Version),
		}
	}
	logger.Error("unsupported WorkchainDescr format")
	return oas.WorkchainDescr{}
}

func convertJettonBridgeParams(logger *zap.Logger, cfg tlb.JettonBridgeParams) oas.JettonBridgeParams {
	switch cfg.SumType {
	case "JettonBridgeParamsV0":
		return oas.JettonBridgeParams{
			BridgeAddress:  ton.AccountID{Workchain: -1, Address: cfg.JettonBridgeParamsV0.BridgeAddress}.ToRaw(),
			OraclesAddress: ton.AccountID{Workchain: -1, Address: cfg.JettonBridgeParamsV0.OraclesAddress}.ToRaw(),
			StateFlags:     int(cfg.JettonBridgeParamsV0.StateFlags),
			BurnBridgeFee:  oas.NewOptInt64(int64(cfg.JettonBridgeParamsV0.BurnBridgeFee)),
			Oracles:        convertOracles(cfg.JettonBridgeParamsV0.Oracles),
		}
	case "JettonBridgeParamsV1":
		return oas.JettonBridgeParams{
			BridgeAddress:        ton.AccountID{Workchain: -1, Address: cfg.JettonBridgeParamsV1.BridgeAddress}.ToRaw(),
			OraclesAddress:       ton.AccountID{Workchain: -1, Address: cfg.JettonBridgeParamsV1.OraclesAddress}.ToRaw(),
			StateFlags:           int(cfg.JettonBridgeParamsV1.StateFlags),
			Oracles:              convertOracles(cfg.JettonBridgeParamsV1.Oracles),
			ExternalChainAddress: oas.NewOptString(cfg.JettonBridgeParamsV1.ExternalChainAddress.Hex()),
			Prices: oas.NewOptJettonBridgePrices(oas.JettonBridgePrices{
				BridgeBurnFee:           int64(cfg.JettonBridgeParamsV1.Prices.BridgeBurnFee),
				BridgeMintFee:           int64(cfg.JettonBridgeParamsV1.Prices.BridgeMintFee),
				WalletMinTonsForStorage: int64(cfg.JettonBridgeParamsV1.Prices.WalletMinTonsForStorage),
				WalletGasConsumption:    int64(cfg.JettonBridgeParamsV1.Prices.WalletGasConsumption),
				MinterMinTonsForStorage: int64(cfg.JettonBridgeParamsV1.Prices.MinterMinTonsForStorage),
				DiscoverGasConsumption:  int64(cfg.JettonBridgeParamsV1.Prices.DiscoverGasConsumption),
			}),
		}
	}
	logger.Error("unsupported JettonBridgeParams format")
	return oas.JettonBridgeParams{}
}

func convertOracles(oracles tlb.HashmapE[tlb.Bits256, tlb.Bits256]) []oas.Oracle {
	result := make([]oas.Oracle, 0, len(oracles.Keys()))
	for _, item := range oracles.Items() {
		result = append(result, oas.Oracle{
			Address:    ton.AccountID{Workchain: -1, Address: item.Key}.ToRaw(),
			SecpPubkey: item.Value.Hex(),
		})
	}
	return result
}

func convertOracleBridgeParams(cfg tlb.OracleBridgeParams) oas.OracleBridgeParams {
	return oas.OracleBridgeParams{
		BridgeAddr:            ton.AccountID{Workchain: -1, Address: cfg.BridgeAddress}.ToRaw(),
		OracleMultisigAddress: ton.AccountID{Workchain: -1, Address: cfg.OracleMutlisigAddress}.ToRaw(),
		ExternalChainAddress:  cfg.ExternalChainAddress.Hex(),
		Oracles:               convertOracles(cfg.Oracles),
	}
}

func convertSizeLimitsConfig(logger *zap.Logger, cfg tlb.SizeLimitsConfig) oas.SizeLimitsConfig {
	switch cfg.SumType {
	case "SizeLimitsConfig":
		return oas.SizeLimitsConfig{
			MaxMsgBits:      int64(cfg.SizeLimitsConfig.MaxMsgBits),
			MaxMsgCells:     int64(cfg.SizeLimitsConfig.MaxMsgCells),
			MaxLibraryCells: int64(cfg.SizeLimitsConfig.MaxLibraryCells),
			MaxVMDataDepth:  int(cfg.SizeLimitsConfig.MaxVmDataDepth),
			MaxExtMsgSize:   int64(cfg.SizeLimitsConfig.MaxExtMsgSize),
			MaxExtMsgDepth:  int(cfg.SizeLimitsConfig.MaxExtMsgDepth),
		}
	case "SizeLimitsConfigV2":
		return oas.SizeLimitsConfig{
			MaxMsgBits:       int64(cfg.SizeLimitsConfigV2.MaxMsgBits),
			MaxMsgCells:      int64(cfg.SizeLimitsConfigV2.MaxMsgCells),
			MaxLibraryCells:  int64(cfg.SizeLimitsConfigV2.MaxLibraryCells),
			MaxVMDataDepth:   int(cfg.SizeLimitsConfigV2.MaxVmDataDepth),
			MaxExtMsgSize:    int64(cfg.SizeLimitsConfigV2.MaxExtMsgSize),
			MaxExtMsgDepth:   int(cfg.SizeLimitsConfigV2.MaxExtMsgDepth),
			MaxAccStateCells: oas.NewOptInt64(int64(cfg.SizeLimitsConfigV2.MaxAccStateCells)),
			MaxAccStateBits:  oas.NewOptInt64(int64(cfg.SizeLimitsConfigV2.MaxAccStateBits)),
		}
	}
	logger.Error("unsupported SizeLimitsConfig format")
	return oas.SizeLimitsConfig{}
}

func convertConsensusConfig(logger *zap.Logger, cfg tlb.ConsensusConfig) oas.BlockchainConfig29 {
	switch cfg.SumType {
	case "ConsensusConfig":
		return oas.BlockchainConfig29{
			RoundCandidates:      int64(cfg.ConsensusConfig.RoundCandidates),
			NextCandidateDelayMs: int64(cfg.ConsensusConfig.NextCandidateDelayMs),
			ConsensusTimeoutMs:   int64(cfg.ConsensusConfig.ConsensusTimeoutMs),
			FastAttempts:         int64(cfg.ConsensusConfig.FastAttempts),
			AttemptDuration:      int64(cfg.ConsensusConfig.AttemptDuration),
			CatchainMaxDeps:      int64(cfg.ConsensusConfig.CatchainMaxDeps),
			MaxBlockBytes:        int64(cfg.ConsensusConfig.MaxBlockBytes),
			MaxCollatedBytes:     int64(cfg.ConsensusConfig.MaxCollatedBytes),
		}
	case "ConsensusConfigNew":
		return oas.BlockchainConfig29{
			Flags:                oas.NewOptInt(int(cfg.ConsensusConfigNew.Flags)),
			NewCatchainIds:       oas.NewOptBool(cfg.ConsensusConfigNew.NewCatchainIds),
			RoundCandidates:      int64(cfg.ConsensusConfigNew.RoundCandidates),
			NextCandidateDelayMs: int64(cfg.ConsensusConfigNew.NextCandidateDelayMs),
			ConsensusTimeoutMs:   int64(cfg.ConsensusConfigNew.ConsensusTimeoutMs),
			FastAttempts:         int64(cfg.ConsensusConfigNew.FastAttempts),
			AttemptDuration:      int64(cfg.ConsensusConfigNew.AttemptDuration),
			CatchainMaxDeps:      int64(cfg.ConsensusConfigNew.CatchainMaxDeps),
			MaxBlockBytes:        int64(cfg.ConsensusConfigNew.MaxBlockBytes),
			MaxCollatedBytes:     int64(cfg.ConsensusConfigNew.MaxCollatedBytes),
		}
	case "ConsensusConfigV3":
		return oas.BlockchainConfig29{
			Flags:                oas.NewOptInt(int(cfg.ConsensusConfigV3.Flags)),
			NewCatchainIds:       oas.NewOptBool(cfg.ConsensusConfigV3.NewCatchainIds),
			RoundCandidates:      int64(cfg.ConsensusConfigV3.RoundCandidates),
			NextCandidateDelayMs: int64(cfg.ConsensusConfigV3.NextCandidateDelayMs),
			ConsensusTimeoutMs:   int64(cfg.ConsensusConfigV3.ConsensusTimeoutMs),
			FastAttempts:         int64(cfg.ConsensusConfigV3.FastAttempts),
			AttemptDuration:      int64(cfg.ConsensusConfigV3.AttemptDuration),
			CatchainMaxDeps:      int64(cfg.ConsensusConfigV3.CatchainMaxDeps),
			MaxBlockBytes:        int64(cfg.ConsensusConfigV3.MaxBlockBytes),
			MaxCollatedBytes:     int64(cfg.ConsensusConfigV3.MaxCollatedBytes),
		}
	case "ConsensusConfigV4":
		return oas.BlockchainConfig29{
			Flags:                  oas.NewOptInt(int(cfg.ConsensusConfigV4.Flags)),
			NewCatchainIds:         oas.NewOptBool(cfg.ConsensusConfigV4.NewCatchainIds),
			RoundCandidates:        int64(cfg.ConsensusConfigV4.RoundCandidates),
			NextCandidateDelayMs:   int64(cfg.ConsensusConfigV4.NextCandidateDelayMs),
			ConsensusTimeoutMs:     int64(cfg.ConsensusConfigV4.ConsensusTimeoutMs),
			FastAttempts:           int64(cfg.ConsensusConfigV4.FastAttempts),
			AttemptDuration:        int64(cfg.ConsensusConfigV4.AttemptDuration),
			CatchainMaxDeps:        int64(cfg.ConsensusConfigV4.CatchainMaxDeps),
			MaxBlockBytes:          int64(cfg.ConsensusConfigV4.MaxBlockBytes),
			MaxCollatedBytes:       int64(cfg.ConsensusConfigV4.MaxCollatedBytes),
			ProtoVersion:           oas.NewOptInt64(int64(cfg.ConsensusConfigV4.ProtoVersion)),
			CatchainMaxBlocksCoeff: oas.NewOptInt64(int64(cfg.ConsensusConfigV4.CatchainMaxBlocksCoeff)),
		}
	}
	logger.Error("unsupported ConsensusConfig format")
	return oas.BlockchainConfig29{}
}

func convertCatchainConfig(logger *zap.Logger, cfg tlb.CatchainConfig) oas.BlockchainConfig28 {
	switch cfg.SumType {
	case "CatchainConfig":
		return oas.BlockchainConfig28{
			McCatchainLifetime:      int64(cfg.CatchainConfig.McCatchainLifetime),
			ShardCatchainLifetime:   int64(cfg.CatchainConfig.ShardCatchainLifetime),
			ShardValidatorsLifetime: int64(cfg.CatchainConfig.ShardValidatorsLifetime),
			ShardValidatorsNum:      int64(cfg.CatchainConfig.ShardValidatorsNum),
		}
	case "CatchainConfigNew":
		return oas.BlockchainConfig28{
			McCatchainLifetime:      int64(cfg.CatchainConfigNew.McCatchainLifetime),
			ShardCatchainLifetime:   int64(cfg.CatchainConfigNew.ShardCatchainLifetime),
			ShardValidatorsLifetime: int64(cfg.CatchainConfigNew.ShardValidatorsLifetime),
			ShardValidatorsNum:      int64(cfg.CatchainConfigNew.ShardValidatorsNum),
			Flags:                   oas.NewOptInt(int(cfg.CatchainConfigNew.Flags)),
			ShuffleMcValidators:     oas.NewOptBool(cfg.CatchainConfigNew.ShuffleMcValidators),
		}
	}
	logger.Error("unsupported CatchainConfig format")
	return oas.BlockchainConfig28{}
}

func convertMsgForwardPrices(prices tlb.MsgForwardPrices) oas.MsgForwardPrices {
	return oas.MsgForwardPrices{
		LumpPrice:      int64(prices.LumpPrice),
		BitPrice:       int64(prices.BitPrice),
		CellPrice:      int64(prices.CellPrice),
		IhrPriceFactor: int64(prices.IhrPriceFactor),
		FirstFrac:      int64(prices.FirstFrac),
		NextFrac:       int64(prices.NextFrac),
	}
}

func convertBlockLimits(limits tlb.BlockLimits) oas.BlockLimits {
	return oas.BlockLimits{
		Bytes: oas.BlockParamLimits{
			Underload: int64(limits.Bytes.Underload),
			SoftLimit: int64(limits.Bytes.SoftLimit),
			HardLimit: int64(limits.Bytes.HardLimit),
		},
		Gas: oas.BlockParamLimits{
			Underload: int64(limits.Gas.Underload),
			SoftLimit: int64(limits.Gas.SoftLimit),
			HardLimit: int64(limits.Gas.HardLimit),
		},
		LtDelta: oas.BlockParamLimits{
			Underload: int64(limits.LtDelta.Underload),
			SoftLimit: int64(limits.LtDelta.SoftLimit),
			HardLimit: int64(limits.LtDelta.HardLimit),
		},
	}
}

func convertPricesAndPricesExt(prices tlb.GasLimitsPrices) (oas.GasLimitPrices, bool) {
	switch prices.SumType {
	case "GasPrices":
		return oas.GasLimitPrices{
			GasPrice:       int64(prices.GasPrices.GasPrice),
			GasLimit:       int64(prices.GasPrices.GasLimit),
			GasCredit:      int64(prices.GasPrices.GasCredit),
			BlockGasLimit:  int64(prices.GasPrices.BlockGasLimit),
			FreezeDueLimit: int64(prices.GasPrices.FreezeDueLimit),
			DeleteDueLimit: int64(prices.GasPrices.DeleteDueLimit),
		}, true
	case "GasPricesExt":
		return oas.GasLimitPrices{
			GasPrice:        int64(prices.GasPricesExt.GasPrice),
			GasLimit:        int64(prices.GasPricesExt.GasLimit),
			GasCredit:       int64(prices.GasPricesExt.GasCredit),
			BlockGasLimit:   int64(prices.GasPricesExt.BlockGasLimit),
			FreezeDueLimit:  int64(prices.GasPricesExt.FreezeDueLimit),
			DeleteDueLimit:  int64(prices.GasPricesExt.DeleteDueLimit),
			SpecialGasLimit: oas.NewOptInt64(int64(prices.GasPricesExt.SpecialGasLimit)),
		}, true
	}
	return oas.GasLimitPrices{}, false
}

func convertGasLimitsPrices(logger *zap.Logger, prices tlb.GasLimitsPrices) oas.GasLimitPrices {
	switch prices.SumType {
	case "GasPrices":
		result, _ := convertPricesAndPricesExt(prices)
		return result
	case "GasPricesExt":
		result, _ := convertPricesAndPricesExt(prices)
		return result
	case "GasFlatPfx":
		other, ok := convertPricesAndPricesExt(*prices.GasFlatPfx.Other)
		if !ok {
			logger.Error("unsupported ConfigParam20/21 format")
			return oas.GasLimitPrices{}
		}
		other.FlatGasPrice = oas.NewOptInt64(int64(prices.GasFlatPfx.FlatGasPrice))
		other.FlatGasLimit = oas.NewOptInt64(int64(prices.GasFlatPfx.FlatGasLimit))
		return other
	}
	logger.Error("unsupported ConfigParam20/21 format")
	return oas.GasLimitPrices{}
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
		s.TotalWeight = oas.NewOptString(fmt.Sprintf("%d", int64(set.ValidatorsExt.TotalWeight)))
	}
	for _, d := range l {
		item := oas.ValidatorsSetListItem{
			PublicKey: d.PubKey().Hex(),
		}
		switch d.SumType {
		case "Validator":
			item.Weight = int64(d.Validator.Weight)
		case "ValidatorAddr":
			item.Weight = int64(d.ValidatorAddr.Weight)
			item.SetAdnlAddr(oas.NewOptString(d.ValidatorAddr.AdnlAddr.Hex()))
		}
		s.List = append(s.List, item)
	}
	return oas.NewOptValidatorsSet(s)
}

func convertActionPhaseResultCode(code int32) *string {
	resultCodes := map[int32]string{
		32:  "Invalid action list format",
		-32: "Method ID not found",
		33:  "Action list too long",
		34:  "Unsupported action",
		35:  "Invalid Source address",
		36:  "Invalid Destination address",
		37:  "Insufficient TON",
		38:  "Insufficient extra-currencies",
		40:  "Insufficient funds",
		43:  "Maximum cells/tree depth exceeded",
	}
	if msg, ok := resultCodes[code]; ok {
		return &msg
	}
	return nil
}
