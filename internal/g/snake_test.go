package g

import "testing"

func TestToSnake(t *testing.T) {
	if CamelToSnake("OloloTrololoV2") != "ololo_trololo_v2" {
		t.Fatal(CamelToSnake("OloloTrololoV2"))
	}
}
