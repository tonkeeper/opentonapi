package api

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-faster/jx"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
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
		tx.InMsg.SetTo(convertMessage(*t.InMsg))
	}
	for _, m := range t.OutMsgs {
		tx.OutMsgs = append(tx.OutMsgs, convertMessage(m))
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

func convertStakingWhalesPool(address tongo.AccountID, w references.WhalesPoolInfo, poolStatus abi.GetStakingStatusResult, poolConfig abi.GetParams_WhalesNominatorResult, apy float64, verified bool) oas.PoolInfo {
	return oas.PoolInfo{
		Address:           address.ToRaw(),
		Name:              w.Name + " " + w.Queue,
		TotalAmount:       int64(poolStatus.StakeSent),
		Implementation:    oas.PoolInfoImplementationWhales,
		Apy:               apy * float64(10000-poolConfig.PoolFee) / 10000,
		MinStake:          poolConfig.MinStake + poolConfig.DepositFee + poolConfig.ReceiptPrice,
		CycleEnd:          int64(poolStatus.StakeUntil),
		CycleStart:        int64(poolStatus.StakeAt),
		Verified:          verified,
		CurrentNominators: 0, //todo: add actual values
		MaxNominators:     40000,
	}
}

func convertStakingTFPool(p core.TFPool, info addressbook.TFPoolInfo, apy float64) oas.PoolInfo {
	name := info.Name
	if name == "" {
		name = "Unknown name ..." + p.Address.ToHuman(true, false)[43:]
	}
	return oas.PoolInfo{
		Address:           p.Address.ToRaw(),
		Name:              name,
		TotalAmount:       p.TotalAmount,
		Implementation:    oas.PoolInfoImplementationTf,
		Apy:               apy * float64(10000-p.ValidatorShare) / 10000,
		MinStake:          p.MinNominatorStake + 1_000_000_000, //this is not in contract. just hardcoded value from documentation
		CycleStart:        int64(p.StakeAt),
		CycleEnd:          int64(p.StakeAt) + 3600*36, //todo: make correct
		Verified:          p.VerifiedSources,
		CurrentNominators: p.Nominators,
		MaxNominators:     p.MaxNominators,
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

func convertToRawAccount(account *core.Account) oas.RawAccount {
	rawAccount := oas.RawAccount{
		Address:           account.AccountAddress.ToRaw(),
		Balance:           account.TonBalance,
		LastTransactionLt: int64(account.LastTransactionLt),
		Status:            account.Status,
		Storage: oas.AccountStorageInfo{
			UsedCells:       account.Storage.UsedCells.Int64(),
			UsedBits:        account.Storage.UsedBits.Int64(),
			UsedPublicCells: account.Storage.UsedPublicCells.Int64(),
			LastPaid:        int64(account.Storage.LastPaid),
			DuePayment:      account.Storage.DuePayment,
		},
	}
	if account.ExtraBalances != nil {
		balances := make(map[string]string, len(account.ExtraBalances))
		for key, value := range account.ExtraBalances {
			balances[fmt.Sprintf("%v", key)] = fmt.Sprintf("%v", value)
		}
		rawAccount.ExtraBalance = oas.NewOptRawAccountExtraBalance(balances)
	}
	if account.Code != nil && len(account.Code) != 0 {
		rawAccount.Code = oas.NewOptString(fmt.Sprintf("%x", account.Code[:]))
	}
	if account.Data != nil {
		rawAccount.Data = oas.NewOptString(fmt.Sprintf("%x", account.Data[:]))
	}
	return rawAccount
}

func convertToAccount(info *core.AccountInfo) oas.Account {
	acc := oas.Account{
		Address:      info.Account.AccountAddress.ToRaw(),
		Balance:      info.Account.TonBalance,
		LastActivity: info.Account.LastActivityTime,
		Status:       info.Account.Status,
		Interfaces:   info.Account.Interfaces,
		GetMethods:   info.Account.GetMethods,
	}
	if info.Name != nil {
		acc.Name = oas.NewOptString(*info.Name)
	}
	if info.Icon != nil {
		acc.Icon = oas.NewOptString(*info.Icon)
	}
	if info.IsScam != nil {
		acc.IsScam = oas.NewOptBool(*info.IsScam)
	}
	if info.MemoRequired != nil {
		acc.MemoRequired = oas.NewOptBool(*info.MemoRequired)
	}
	return acc
}

func convertToApiJetton(metadata tongo.JettonMetadata, master tongo.AccountID, imgGenerator previewGenerator) (oas.Jetton, error) {
	convertVerification, _ := convertJettonVerification(addressbook.None) // TODO: change to real verify
	name := metadata.Name
	if name == "" {
		name = "Unknown Token"
	}
	symbol := metadata.Symbol
	if symbol == "" {
		symbol = "UKWN"
	}
	normalizedSymbol := strings.TrimSpace(strings.ToUpper(symbol))
	if normalizedSymbol == "TON" || normalizedSymbol == "TОN" { //eng and russian
		symbol = "SCAM"
	}
	jetton := oas.Jetton{
		Address:      master.ToRaw(),
		Name:         name,
		Symbol:       symbol,
		Verification: oas.OptJettonVerificationType{Value: convertVerification},
	}
	dec, err := convertJettonDecimals(metadata.Decimals)
	if err != nil {
		return oas.Jetton{}, err
	}
	jetton.Decimals = dec
	if metadata.Image != "" {
		preview := imgGenerator.GenerateImageUrl(metadata.Image, 200, 200)
		jetton.Image = oas.OptString{Value: preview}
	}
	return jetton, nil
}

func convertJettonDecimals(decimals string) (int, error) {
	if decimals == "" {
		return 9, nil
	}
	dec, err := strconv.Atoi(decimals)
	if err != nil {
		return 0, err
	}
	return dec, nil
}

func convertJettonVerification(verificationType addressbook.JettonVerificationType) (oas.JettonVerificationType, error) {
	switch verificationType {
	case addressbook.Whitelist:
		return oas.JettonVerificationTypeWhitelist, nil
	case addressbook.Blacklist:
		return oas.JettonVerificationTypeBlacklist, nil
	case addressbook.None:
		return oas.JettonVerificationTypeNone, nil
	default:
		// if we do not find matches, then we throw out an error and set a default api.JettonVerificationTypeNone
		return oas.JettonVerificationTypeNone, fmt.Errorf("convert jetton verification error: %v", verificationType)
	}
}

func rewriteIfNotEmpty(src, dest string) string {
	if dest != "" {
		return dest
	}
	return src
}

func convertAction(a bath.Action) oas.Action {

	action := oas.Action{
		Type: oas.ActionType(a.Type),
	}
	if a.Success {
		action.Status = oas.ActionStatusOk
	} else {
		action.Status = oas.ActionStatusFailed
	}
	switch a.Type {
	case bath.TonTransfer:
		action.TonTransfer.SetTo(oas.TonTransferAction{
			Amount:    a.TonTransfer.Amount,
			Comment:   pointerToOptString(a.TonTransfer.Comment),
			Payload:   pointerToOptString(a.TonTransfer.Payload),
			Recipient: convertAccountAddress(a.TonTransfer.Recipient),
			Sender:    convertAccountAddress(a.TonTransfer.Sender),
		})
		if a.TonTransfer.Refund != nil {
			action.TonTransfer.Value.Refund.SetTo(oas.Refund{
				Type:   oas.RefundType(a.TonTransfer.Refund.Type),
				Origin: a.TonTransfer.Refund.Origin,
			})

		}

	case bath.NftTransfer:
		action.NftItemTransfer.SetTo(oas.NftItemTransferAction{
			Nft:       a.NftTransfer.Nft.ToRaw(),
			Recipient: convertOptAccountAddress(a.NftTransfer.Recipient),
			Sender:    convertOptAccountAddress(a.NftTransfer.Sender),
		})
	case bath.JettonTransfer:
		action.JettonTransfer.SetTo(oas.JettonTransferAction{
			Amount:           a.JettonTransfer.Amount.String(),
			Recipient:        convertOptAccountAddress(a.JettonTransfer.Recipient),
			Sender:           convertOptAccountAddress(a.JettonTransfer.Sender),
			RecipientsWallet: a.JettonTransfer.RecipientsWallet.ToRaw(),
			SendersWallet:    a.JettonTransfer.SendersWallet.ToRaw(),
			Comment:          pointerToOptString(a.JettonTransfer.Comment),
		})
	case bath.Subscription:
		action.Subscribe.SetTo(oas.SubscriptionAction{
			Amount:       a.Subscription.Amount,
			Beneficiary:  convertAccountAddress(a.Subscription.Beneficiary),
			Subscriber:   convertAccountAddress(a.Subscription.Subscriber),
			Subscription: a.Subscription.Subscription.ToRaw(),
			Initial:      a.Subscription.First,
		})
	case bath.UnSubscription:
		action.UnSubscribe.SetTo(oas.UnSubscriptionAction{
			Beneficiary:  convertAccountAddress(a.UnSubscription.Beneficiary),
			Subscriber:   convertAccountAddress(a.UnSubscription.Subscriber),
			Subscription: a.UnSubscription.Subscription.ToRaw(),
		})
	case bath.ContractDeploy:
		action.ContractDeploy.SetTo(oas.ContractDeployAction{
			Address:    a.ContractDeploy.Address.ToRaw(),
			Interfaces: a.ContractDeploy.Interfaces,
			Deployer:   convertAccountAddress(a.ContractDeploy.Sender),
		})

	}
	return action
}

func convertFees(fee bath.Fee) oas.Fee {
	return oas.Fee{
		Account: convertAccountAddress(fee.WhoPay),
		Total:   0,
		Gas:     fee.Compute,
		Rent:    fee.Storage,
		Deposit: fee.Deposit,
		Refund:  0,
	}
}

func convertTvmStackValue(v tlb.VmStackValue) (oas.TvmStackRecord, error) {
	//	VmStkTuple   VmStkTuple    `tlbSumType:"vm_stk_tuple#07"`
	switch v.SumType {
	case "VmStkNull":
		return oas.TvmStackRecord{Type: oas.TvmStackRecordTypeNull}, nil
	case "VmStkNan":
		return oas.TvmStackRecord{Type: oas.TvmStackRecordTypeNan}, nil
	case "VmStkTinyInt":
		str := fmt.Sprintf("0x%x", v.VmStkTinyInt)
		if v.VmStkTinyInt < 0 {
			str = "-0x" + str[3:]
		}
		return oas.TvmStackRecord{Type: oas.TvmStackRecordTypeNum, Num: oas.NewOptString(str)}, nil
	case "VmStkInt":
		b := big.Int(v.VmStkInt)
		return oas.TvmStackRecord{Type: oas.TvmStackRecordTypeNum, Num: oas.NewOptString(fmt.Sprintf("0x%x", b.Bytes()))}, nil //todo: fix negative
	case "VmStkCell":
		boc, err := v.VmStkCell.Value.ToBocString()
		if err != nil {
			return oas.TvmStackRecord{}, err
		}
		return oas.TvmStackRecord{Type: oas.TvmStackRecordTypeCell, Cell: oas.NewOptString(boc)}, nil
	case "VmStkSlice":
		boc, err := v.VmStkSlice.Cell().ToBocString()
		if err != nil {
			return oas.TvmStackRecord{}, err
		}
		return oas.TvmStackRecord{Type: oas.TvmStackRecordTypeCell, Cell: oas.NewOptString(boc)}, nil
	default:
		return oas.TvmStackRecord{}, fmt.Errorf("can't conver %v stack to rest json", v.SumType)
	}
}

func stringToTVMStackRecord(s string) (tlb.VmStackValue, error) {
	if s == "" {
		return tlb.VmStackValue{}, fmt.Errorf("zero length sting can't be converted to tvm stack")
	}
	if s == "NaN" {
		return tlb.VmStackValue{SumType: "VmStkNan"}, nil
	}
	if s == "Null" {
		return tlb.VmStackValue{SumType: "VmStkNull"}, nil
	}
	a, err := tongo.ParseAccountID(s)
	if err == nil {
		return tlb.TlbStructToVmCellSlice(a.ToMsgAddress())
	}
	if strings.HasPrefix(s, "0x") {
		b, err := hex.DecodeString(s[2:])
		if err != nil {
			return tlb.VmStackValue{}, err
		}
		i := big.Int{}
		i.SetBytes(b)
		return tlb.VmStackValue{SumType: "VmStkInt", VmStkInt: tlb.Int257(i)}, nil
	}
	isDigit := true
	for _, c := range s {
		if !unicode.IsDigit(c) {
			isDigit = false
			break
		}
	}
	if isDigit {
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return tlb.VmStackValue{}, err
		}
		return tlb.VmStackValue{SumType: "VmStkTinyInt", VmStkTinyInt: i}, nil
	}
	c, err := boc.DeserializeSinglRootBase64(s)
	if err != nil {
		return tlb.VmStackValue{}, err
	}
	return tlb.VmStackValue{SumType: "VmStkCell", VmStkCell: tlb.Ref[boc.Cell]{Value: *c}}, nil
}
