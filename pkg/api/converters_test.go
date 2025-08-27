package api

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
)

func TestConverter_DecodeTupleMatrix(t *testing.T) {
	tests := []struct {
		name   string
		boc    string
		output oas.TvmStackRecord
	}{
		{
			name: "tuple 0x0",
			boc:  "b5ee9c7201010301001100020c000001070001010200000006070000",
			output: oas.TvmStackRecord{
				Type: oas.TvmStackRecordTypeTuple,
				Tuple: []oas.TvmStackRecord{
					{
						Type:  oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{},
					},
				},
			},
		},
		{
			name: "tuple 0x1",
			boc:  "b5ee9c7201010401001d00020c000001070001010200000106070001030012010000000000000001",
			output: oas.TvmStackRecord{
				Type: oas.TvmStackRecordTypeTuple,
				Tuple: []oas.TvmStackRecord{
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x1"),
							},
						},
					},
				},
			},
		},
		{
			name: "tuple 0x2",
			boc:  "b5ee9c7201010501002900020c000001070001010200000206070002030400120100000000000000010012010000000000000002",
			output: oas.TvmStackRecord{
				Type: oas.TvmStackRecordTypeTuple,
				Tuple: []oas.TvmStackRecord{
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x1"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x2"),
							},
						},
					},
				},
			},
		},
		{
			name: "tuple 0x3",
			boc:  "b5ee9c7201010701003800020c000001070001010200000206070003030402000506001201000000000000000300120100000000000000010012010000000000000002",
			output: oas.TvmStackRecord{
				Type: oas.TvmStackRecordTypeTuple,
				Tuple: []oas.TvmStackRecord{
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x1"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x2"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x3"),
							},
						},
					},
				},
			},
		},
		{
			name: "tuple 1x0",
			boc:  "b5ee9c7201010301001200030c00000107000201020200000006070000",
			output: oas.TvmStackRecord{
				Type: oas.TvmStackRecordTypeTuple,
				Tuple: []oas.TvmStackRecord{
					{
						Type:  oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{},
					},
					{
						Type:  oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{},
					},
				},
			},
		},
		{
			name: "tuple 1x1",
			boc:  "b5ee9c7201010601002f00030c000001070002010203000001060700010401060700010500120100000000000000010012010000000000000002",
			output: oas.TvmStackRecord{
				Type: oas.TvmStackRecordTypeTuple,
				Tuple: []oas.TvmStackRecord{
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x1"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x2"),
							},
						},
					},
				},
			},
		},
		{
			name: "tuple 1x2",
			boc:  "b5ee9c7201010801004700030c000001070002010203000002060700020405020607000206070012010000000000000001001201000000000000000200120100000000000000030012010000000000000004",
			output: oas.TvmStackRecord{
				Type: oas.TvmStackRecordTypeTuple,
				Tuple: []oas.TvmStackRecord{
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x1"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x2"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x3"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x4"),
							},
						},
					},
				},
			},
		},
		{
			name: "tuple 1x3",
			boc:  "b5ee9c7201010c01006500030c0000010700020102030000020607000304050206070003060702000809001201000000000000000302000a0b00120100000000000000060012010000000000000001001201000000000000000200120100000000000000040012010000000000000005",
			output: oas.TvmStackRecord{
				Type: oas.TvmStackRecordTypeTuple,
				Tuple: []oas.TvmStackRecord{
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x1"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x2"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x3"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x4"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x5"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x6"),
							},
						},
					},
				},
			},
		},
		{
			name: "tuple 2x0",
			boc:  "b5ee9c7201010401001600030c0000010700030102030000020003030006070000",
			output: oas.TvmStackRecord{
				Type: oas.TvmStackRecordTypeTuple,
				Tuple: []oas.TvmStackRecord{
					{
						Type:  oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{},
					},
					{
						Type:  oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{},
					},
					{
						Type:  oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{},
					},
				},
			},
		},
		{
			name: "tuple 2x1",
			boc:  "b5ee9c7201010901004400030c000001070003010203000002000405010607000106010607000107010607000108001201000000000000000300120100000000000000010012010000000000000002",
			output: oas.TvmStackRecord{
				Type: oas.TvmStackRecordTypeTuple,
				Tuple: []oas.TvmStackRecord{
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x1"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x2"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x3"),
							},
						},
					},
				},
			},
		},
		{
			name: "tuple 2x2",
			boc:  "b5ee9c7201010c01006800030c000001070003010203000002000405020607000206070206070002080902060700020a0b001201000000000000000500120100000000000000060012010000000000000001001201000000000000000200120100000000000000030012010000000000000004",
			output: oas.TvmStackRecord{
				Type: oas.TvmStackRecordTypeTuple,
				Tuple: []oas.TvmStackRecord{
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x1"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x2"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x3"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x4"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x5"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x6"),
							},
						},
					},
				},
			},
		},
		{
			name: "tuple 2x3",
			boc:  "b5ee9c7201011201009500030c000001070003010203000002000405020607000306070206070003080902060700030a0b02000c0d001201000000000000000902000e0f0012010000000000000003020010110012010000000000000006001201000000000000000700120100000000000000080012010000000000000001001201000000000000000200120100000000000000040012010000000000000005",
			output: oas.TvmStackRecord{
				Type: oas.TvmStackRecordTypeTuple,
				Tuple: []oas.TvmStackRecord{
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x1"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x2"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x3"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x4"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x5"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x6"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x7"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x8"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x9"),
							},
						},
					},
				},
			},
		},
		{
			name: "tuple 3x0",
			boc:  "b5ee9c7201010501001a00030c000001070004010204000002000304020004040006070000",
			output: oas.TvmStackRecord{
				Type: oas.TvmStackRecordTypeTuple,
				Tuple: []oas.TvmStackRecord{
					{
						Type:  oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{},
					},
					{
						Type:  oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{},
					},
					{
						Type:  oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{},
					},
					{
						Type:  oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{},
					},
				},
			},
		},
		{
			name: "tuple 3x1",
			boc:  "b5ee9c7201010c01005900030c00000107000401020300000200040501060700010602000708010607000109001201000000000000000401060700010a01060700010b001201000000000000000300120100000000000000010012010000000000000002",
			output: oas.TvmStackRecord{
				Type: oas.TvmStackRecordTypeTuple,
				Tuple: []oas.TvmStackRecord{
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x1"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x2"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x3"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x4"),
							},
						},
					},
				},
			},
		},
		{
			name: "tuple 3x2",
			boc:  "b5ee9c7201011001008900030c000001070004010203000002000405020607000206070200080902060700020a0b0012010000000000000007001201000000000000000802060700020c0d02060700020e0f001201000000000000000500120100000000000000060012010000000000000001001201000000000000000200120100000000000000030012010000000000000004",
			output: oas.TvmStackRecord{
				Type: oas.TvmStackRecordTypeTuple,
				Tuple: []oas.TvmStackRecord{
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x1"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x2"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x3"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x4"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x5"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x6"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x7"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x8"),
							},
						},
					},
				},
			},
		},
		{
			name: "tuple 3x3",
			boc:  "b5ee9c720101180100c500030c000001070004010203000002000405020607000306070200080902060700030a0b02000c0d001201000000000000000c02060700030e0f02060700031011020012130012010000000000000009001201000000000000000a001201000000000000000b020014150012010000000000000003020016170012010000000000000006001201000000000000000700120100000000000000080012010000000000000001001201000000000000000200120100000000000000040012010000000000000005",
			output: oas.TvmStackRecord{
				Type: oas.TvmStackRecordTypeTuple,
				Tuple: []oas.TvmStackRecord{
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x1"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x2"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x3"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x4"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x5"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x6"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x7"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x8"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0x9"),
							},
						},
					},
					{
						Type: oas.TvmStackRecordTypeTuple,
						Tuple: []oas.TvmStackRecord{
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0xa"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0xb"),
							},
							{
								Type: oas.TvmStackRecordTypeNum,
								Num:  oas.NewOptString("0xc"),
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stackCell, err := boc.DeserializeSinglRootHex(tt.boc)
			require.Nil(t, err)
			var stack tlb.VmStack
			if err := tlb.Unmarshal(stackCell, &stack); err != nil {
				t.Fatal(err)
			}
			value, err := convertTvmStackValue(stack[0])
			if err != nil {
				t.Error(err)
			}
			if !compareNumTuples(&value, &tt.output) {
				t.Errorf("tuples are different")
			}
		})
	}
}

func compareNumTuples(actual *oas.TvmStackRecord, expected *oas.TvmStackRecord) bool {
	if actual.Type != expected.Type {
		return false
	}
	if actual.Type == oas.TvmStackRecordTypeTuple {
		sumResult := true
		for i := range actual.Tuple {
			sumResult = sumResult && compareNumTuples(&actual.Tuple[i], &expected.Tuple[i])
		}

		return sumResult
	} else {
		return actual.Num == expected.Num
	}
}
