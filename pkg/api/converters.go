package api

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-faster/jx"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
)

func anyToJSONRawMap(a any, toSnake bool) map[string]jx.Raw { //todo: переписать этот ужас
	var m = map[string]jx.Raw{}
	if am, ok := a.(map[string]any); ok {
		for k, v := range am {
			if toSnake {
				k = g.CamelToSnake(k)
			}
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
			name := t.Type().Field(i).Name
			if toSnake {
				name = g.CamelToSnake(name)
			}
			m[name] = b
		}
	default:
		panic(fmt.Sprintf("some shit %v", t.Kind()))
	}
	return m
}

func convertAccountAddress(id tongo.AccountID, book addressBook) oas.AccountAddress {
	i, prs := book.GetAddressInfoByAddress(id)
	address := oas.AccountAddress{Address: id.ToRaw()}
	if prs {
		if i.Name != "" {
			address.SetName(oas.NewOptString(i.Name))
		}
		if i.Image != "" {
			address.SetIcon(oas.NewOptString(i.Image))
		}
		address.IsScam = i.IsScam
	}
	return address
}

func convertOptAccountAddress(id *tongo.AccountID, book addressBook) oas.OptAccountAddress {
	if id != nil {
		return oas.OptAccountAddress{Value: convertAccountAddress(*id, book), Set: true}
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
