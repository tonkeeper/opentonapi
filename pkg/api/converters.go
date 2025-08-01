package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	imgGenerator "github.com/tonkeeper/opentonapi/pkg/image"
	"github.com/tonkeeper/opentonapi/pkg/references"

	"github.com/go-faster/jx"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"google.golang.org/grpc/status"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	walletPkg "github.com/tonkeeper/opentonapi/pkg/wallet"
)

// ErrorWithExtendedCode helps to pass additional information about an error.
type ErrorWithExtendedCode struct {
	Code         int
	Message      string
	ExtendedCode references.ExtendedCode
}

func (e ErrorWithExtendedCode) Error() string {
	return e.Message
}

// censor removes sensitive information from the error message.
func censor(msg string) string {
	if strings.HasPrefix(msg, "failed to connect to") || strings.Contains(msg, "host=") {
		return "unknown error"
	}
	return msg
}

func extendedCode(code references.ExtendedCode) oas.OptInt64 {
	if code == 0 {
		return oas.OptInt64{}
	}
	return oas.NewOptInt64(int64(code))
}

func toError(defaultCode int, err error) *oas.ErrorStatusCode {
	var e ErrorWithExtendedCode
	if errors.As(err, &e) {
		return &oas.ErrorStatusCode{
			StatusCode: e.Code,
			Response: oas.Error{
				Error:     censor(e.Message),
				ErrorCode: extendedCode(e.ExtendedCode),
			},
		}
	}
	if s, ok := status.FromError(err); ok {
		return &oas.ErrorStatusCode{StatusCode: defaultCode, Response: oas.Error{Error: censor(s.Message())}}
	}
	return &oas.ErrorStatusCode{StatusCode: defaultCode, Response: oas.Error{Error: censor(err.Error())}}
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
			var b []byte
			var err error
			if aj, ok := t.Field(i).Interface().(json.Marshaler); ok {
				b, err = aj.MarshalJSON()
			} else if t.Field(i).Kind() == reflect.Struct {
				m := anyToJSONRawMap(t.Field(i).Interface())
				m2 := make(map[string]json.RawMessage)
				for k, v := range m {
					m2[k] = json.RawMessage(v)
				}
				b, err = json.Marshal(m2)
			} else {
				b, err = json.Marshal(t.Field(i).Interface())
			}
			if err != nil {
				panic("some shit")
			}
			name := t.Type().Field(i).Name
			m[name] = b
		}
	default:
		panic(fmt.Sprintf("some shit %v", t.Kind()))
	}
	return m
}

var NoneAccount = oas.AccountAddress{
	Address: "",
	Name:    oas.NewOptString("NoneAddr"),
}

func convertAccountAddress(id tongo.AccountID, book addressBook) oas.AccountAddress {
	address := oas.AccountAddress{Address: id.ToRaw()}
	if i, prs := book.GetAddressInfoByAddress(id); prs {
		if i.Name != "" {
			address.SetName(oas.NewOptString(i.Name))
		}
		if i.Image != "" {
			address.SetIcon(oas.NewOptString(imgGenerator.DefaultGenerator.GenerateImageUrl(i.Image, 200, 200)))
		}
		address.IsScam = i.IsScam
	}
	if wallet, err := book.IsWallet(id); err == nil {
		address.IsWallet = wallet
	}
	return address
}

func optIntToPointer(o oas.OptInt64) *int64 {
	if !o.IsSet() {
		return nil
	}
	return &o.Value
}

func convertOptAccountAddress(id *tongo.AccountID, book addressBook) oas.OptAccountAddress {
	if id != nil {
		return oas.OptAccountAddress{Value: convertAccountAddress(*id, book), Set: true}
	}
	return oas.OptAccountAddress{}
}

func rewriteIfNotEmpty(src, dest string) string {
	if dest != "" {
		return dest
	}
	return src
}

func convertTvmStackValue(v tlb.VmStackValue) (oas.TvmStackRecord, error) {
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
		str := fmt.Sprintf("0x%x", b.Bytes())
		if b.Sign() == -1 {
			str = "-" + str
		}
		return oas.TvmStackRecord{Type: oas.TvmStackRecordTypeNum, Num: oas.NewOptString(str)}, nil
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
	case "VmStkTuple":
		return convertTuple(v.VmStkTuple)
	default:
		return oas.TvmStackRecord{}, fmt.Errorf("can't conver %v stack to rest json", v.SumType)
	}
}

func convertTuple(v tlb.VmStkTuple) (oas.TvmStackRecord, error) {
	var records []tlb.VmStackValue
	var err error
	r := oas.TvmStackRecord{Type: oas.TvmStackRecordTypeTuple}
	if v.Len == 0 {
		return r, nil
	}
	if v.Len == 2 && (v.Data.Tail.SumType == "VmStkTuple" || v.Data.Tail.SumType == "VmStkNull") {
		records, err = v.RecursiveToSlice()
	} else {
		records, err = v.Data.RecursiveToSlice(int(v.Len))
	}
	if err != nil {
		return r, err
	}
	for _, v := range records {
		ov, err := convertTvmStackValue(v)
		if err != nil {
			return r, err
		}
		r.Tuple = append(r.Tuple, ov)
	}
	return r, nil
}

func parseExecGetMethodArgs(arg oas.ExecGetMethodArg) (tlb.VmStackValue, error) {
	switch arg.Type {
	case oas.ExecGetMethodArgTypeNan:
		if arg.Value != "NaN" {
			return tlb.VmStackValue{}, fmt.Errorf("expected 'NaN' for type 'nan', got '%v'", arg.Value)
		}
		return tlb.VmStackValue{SumType: "VmStkNan"}, nil

	case oas.ExecGetMethodArgTypeNull:
		if arg.Value != "Null" {
			return tlb.VmStackValue{}, fmt.Errorf("expected 'Null' for type 'null', got '%v'", arg.Value)
		}
		return tlb.VmStackValue{SumType: "VmStkNull"}, nil

	case oas.ExecGetMethodArgTypeTinyint:
		i, err := strconv.ParseInt(arg.Value, 10, 64)
		if err != nil {
			return tlb.VmStackValue{}, fmt.Errorf("invalid tinyint value: %v", err)
		}
		return tlb.VmStackValue{SumType: "VmStkTinyInt", VmStkTinyInt: i}, nil

	case oas.ExecGetMethodArgTypeInt257:
		if !strings.HasPrefix(arg.Value, "0x") {
			return tlb.VmStackValue{}, fmt.Errorf("int257 value must start with '0x'")
		}
		i := big.Int{}
		if _, ok := i.SetString(arg.Value[2:], 16); !ok {
			return tlb.VmStackValue{}, fmt.Errorf("invalid int257 hex: %v", arg.Value)
		}
		return tlb.VmStackValue{SumType: "VmStkInt", VmStkInt: tlb.Int257(i)}, nil

	case oas.ExecGetMethodArgTypeSlice:
		account, err := tongo.ParseAddress(arg.Value)
		if err != nil {
			return tlb.VmStackValue{}, fmt.Errorf("invalid address: %v", err)
		}
		return tlb.TlbStructToVmCellSlice(account.ID.ToMsgAddress())

	case oas.ExecGetMethodArgTypeCellBocBase64:
		c, err := boc.DeserializeSinglRootBase64(arg.Value)
		if err != nil {
			return tlb.VmStackValue{}, fmt.Errorf("invalid cell BOC base64: %v", err)
		}
		return tlb.VmStackValue{SumType: "VmStkCell", VmStkCell: tlb.Ref[boc.Cell]{Value: *c}}, nil

	case oas.ExecGetMethodArgTypeSliceBocHex:
		cells, err := boc.DeserializeBocHex(arg.Value)
		if err != nil || len(cells) != 1 {
			return tlb.VmStackValue{}, fmt.Errorf("invalid slice BOC hex: %v", err)
		}
		return tlb.CellToVmCellSlice(cells[0])

	default:
		return tlb.VmStackValue{}, fmt.Errorf("unsupported argument type: %v", arg.Type)
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
	account, err := tongo.ParseAddress(s)
	if err == nil {
		return tlb.TlbStructToVmCellSlice(account.ID.ToMsgAddress())
	}
	if strings.HasPrefix(s, "0x") {
		i := big.Int{}
		_, ok := i.SetString(s[2:], 16)
		if !ok {
			return tlb.VmStackValue{}, fmt.Errorf("invalid hex %v", s)
		}
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
	cells, err := boc.DeserializeBocHex(s)
	if err == nil && len(cells) == 1 {
		return tlb.CellToVmCellSlice(cells[0])
	}
	c, err := boc.DeserializeSinglRootBase64(s)
	if err != nil {
		return tlb.VmStackValue{}, err
	}
	return tlb.VmStackValue{SumType: "VmStkCell", VmStkCell: tlb.Ref[boc.Cell]{Value: *c}}, nil
}

func (h *Handler) convertMultisig(ctx context.Context, item core.Multisig) (*oas.Multisig, error) {
	converted := oas.Multisig{
		Address:   item.AccountID.ToRaw(),
		Seqno:     item.Seqno.String(),
		Threshold: item.Threshold,
	}
	for _, account := range item.Signers {
		converted.Signers = append(converted.Signers, account.ToRaw())
	}
	for _, account := range item.Proposers {
		converted.Proposers = append(converted.Proposers, account.ToRaw())
	}
	for _, order := range item.Orders {
		o, err := h.convertMultisigOrder(ctx, order)
		if err != nil {
			return nil, err
		}
		converted.Orders = append(converted.Orders, o)
	}
	return &converted, nil
}

func (h *Handler) convertMultisigOrder(ctx context.Context, order core.MultisigOrder) (oas.MultisigOrder, error) {
	var signers []string
	for _, account := range order.Signers {
		signers = append(signers, account.ToRaw())
	}
	risk := walletPkg.Risk{
		TransferAllRemainingBalance: false,
		Jettons:                     map[tongo.AccountID]big.Int{},
	}
	var cp oas.OptMultisigOrderChangingParameters
	for _, action := range order.Actions {
		switch action.SumType {
		case "SendMessage":
			var err error
			risk, err = walletPkg.ExtractRiskFromMessage(action.SendMessage.Field0.Message, risk, action.SendMessage.Field0.Mode)
			if err != nil {
				return oas.MultisigOrder{}, err
			}
		case "UpdateMultisigParam":
			newParams := oas.MultisigOrderChangingParameters{
				Threshold: int32(action.UpdateMultisigParam.Threshold),
			}
			for _, s := range action.UpdateMultisigParam.Signers.Values() {
				a, err := tongo.AccountIDFromTlb(s)
				if err != nil || a == nil {
					return oas.MultisigOrder{}, fmt.Errorf("can't convert %v to account id", s)
				}
				newParams.Signers = append(newParams.Signers, a.ToRaw())
			}
			for _, p := range action.UpdateMultisigParam.Proposers.Values() {
				a, err := tongo.AccountIDFromTlb(p)
				if err != nil || a == nil {
					return oas.MultisigOrder{}, fmt.Errorf("can't convert %v to account id", p)
				}
				newParams.Proposers = append(newParams.Proposers, a.ToRaw())
			}
			cp.SetTo(newParams)
		}
	}
	oasRisk, err := h.convertRisk(ctx, risk, order.MultisigAccountID)
	if err != nil {
		return oas.MultisigOrder{}, err
	}

	return oas.MultisigOrder{
		MultisigAddress:    order.MultisigAccountID.ToRaw(),
		Address:            order.AccountID.ToRaw(),
		OrderSeqno:         order.OrderSeqno.String(),
		Threshold:          order.Threshold,
		SentForExecution:   order.SentForExecution,
		Signers:            signers,
		ApprovalsNum:       order.ApprovalsNum,
		ExpirationDate:     order.ExpirationDate,
		CreationDate:       order.CreationDate,
		Risk:               oasRisk,
		ChangingParameters: cp,
	}, nil
}
