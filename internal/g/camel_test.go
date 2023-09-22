package g

import (
	"encoding/json"
	"testing"
)

var cases = []struct {
	name, input, output string
}{
	{"empty", "", ""},
	{"empty2", "[]", "[]"},
	{"empty3", "{}", "{}"},
	{"empty4", `""`, `""`},
	{"string", `"a"`, `"a"`},
	{"simple", `{"A":["B"],"C":1,"D":"e"}`, `{"a":["B"],"c":1,"d":"e"}`},
	{"hard", `{"A":["b","c"],"C":10.11,"D":"E","F":{"G":"h"}}`, `{"a":["b","c"],"c":10.11,"d":"E","f":{"g":"h"}}`},
	{"hard2", `{"A":["b","c"],"C":0.25,"D":null,"F":{"G":"h","a":[{"O":"o"},{"D":1},["a",{"C":"d"}]]}}`, `{"a":["b","c"],"c":0.25,"d":null,"f":{"g":"h","a":[{"o":"o"},{"d":1},["a",{"c":"d"}]]}}`},
	{"real",
		`{"query_id":3411388337375148,"amount":"1500000000000","Sender":"0:2cf3b5b8c891e517c9addbda1c0386a09ccacbb0e3faf630b51cfc8152325acb","forward_payload":{"is_right":true,"value":{"SumType": "StonfiSwap","OpCode":630424929,"Value":{"TokenWallet":"0:14ac072c56291232d7cd93ddec120235c5e5cf5e2027f49bbc5aa276e5d224d8","MinOut":"2839676791","ToAddress":"0:2cf3b5b8c891e517c9addbda1c0386a09ccacbb0e3faf630b51cfc8152325acb","ReferralAddress":null}}}}`,
		`{"query_id":3411388337375148,"amount":"1500000000000","sender":"0:2cf3b5b8c891e517c9addbda1c0386a09ccacbb0e3faf630b51cfc8152325acb","forward_payload":{"is_right":true,"value":{"sum_type":"StonfiSwap","op_code":630424929,"value":{"token_wallet":"0:14ac072c56291232d7cd93ddec120235c5e5cf5e2027f49bbc5aa276e5d224d8","min_out":"2839676791","to_address":"0:2cf3b5b8c891e517c9addbda1c0386a09ccacbb0e3faf630b51cfc8152325acb","referral_address":null}}}}`,
	},
}

func TestConverter(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ChangeJsonKeys([]byte(c.input), CamelToSnake)
			if string(got) != c.output {
				t.Errorf("got:\n%s\n, want:\n%s", string(got), c.output)
			}
		})
	}
}

func BenchmarkConverter(b *testing.B) {
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ChangeJsonKeys([]byte(c.input), CamelToSnake)
			}
		})
	}
}

func BenchmarkNaiveConverter(b *testing.B) {
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				naiveChangeKeys([]byte(c.input), CamelToSnake)
			}
		})
	}
}

func naiveChangeKeys(j json.RawMessage, fixKey func(s string) string) json.RawMessage {
	m := make(map[string]json.RawMessage)
	if err := json.Unmarshal(j, &m); err != nil {
		// Not a JSON object
		return j
	}

	for k, v := range m {
		fixed := fixKey(k)
		delete(m, k)
		m[fixed] = naiveChangeKeys(v, fixKey)
	}

	b, err := json.Marshal(m)
	if err != nil {
		return j
	}

	return b
}
