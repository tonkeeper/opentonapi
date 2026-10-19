package rates

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// poolStack builds a get_pool_full_data result with `fields` entries, the
// last two being projected_balance and projected_supply.
func poolStack(t *testing.T, fields int, balance, supply string) ExecutionResult {
	t.Helper()
	entries := make([]string, 0, fields)
	for i := 0; i < fields-2; i++ {
		entries = append(entries, `{"type":"num","num":"0x1"}`)
	}
	entries = append(entries,
		fmt.Sprintf(`{"type":"num","num":%q}`, balance),
		fmt.Sprintf(`{"type":"num","num":%q}`, supply),
	)
	var result ExecutionResult
	body := `{"success":true,"exit_code":0,"stack":[` + strings.Join(entries, ",") + `]}`
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestProjectedRatioFromStack(t *testing.T) {
	// KTON's pool (34 fields) as TonAPI returned it on 2026-10-18.
	price, err := projectedRatioFromStack(poolStack(t, 34, "0x68e0e12794c1b", "0x61182242d6d48"))
	if err != nil {
		t.Fatal(err)
	}
	if price < 1.0801 || price > 1.0803 {
		t.Fatalf("KTON price = %v, want about 1.0802", price)
	}
	// The Tonstakers layout (30 fields) reads the same way.
	price, err = projectedRatioFromStack(poolStack(t, 30, "0x2", "0x1"))
	if err != nil || price != 2 {
		t.Fatalf("30 fields: price = %v, err = %v", price, err)
	}
	for name, result := range map[string]ExecutionResult{
		"zero supply":  poolStack(t, 34, "0x1", "0x0"),
		"not a number": poolStack(t, 34, "0x1", "x"),
		"failed":       {Success: false},
	} {
		if _, err := projectedRatioFromStack(result); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}
