package api

import (
	"context"
	"testing"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo/tlb"
)

// TestStonfiNilAddressNoPanic is the regression test for the getAdditionalInfoStonfi
// nil-pointer dereference: ton.AccountIDFromTlb returns (nil, nil) for addr_none /
// addr_extern, so a STONfi pool reporting an addr_none token address used to panic
// the serving goroutine. After the guard, the function returns gracefully without
// setting STONfiPool. (On the unpatched code this test panics.)
func TestStonfiNilAddressNoPanic(t *testing.T) {
	none := tlb.MsgAddress{SumType: "AddrNone"}
	info := &core.TraceAdditionalInfo{}

	// must not panic on addr_none token addresses
	getAdditionalInfoStonfi(context.Background(), nil, info, none, none)

	if info.STONfiPool != nil {
		t.Fatalf("expected STONfiPool to be unset when token addresses are addr_none, got %+v", info.STONfiPool)
	}
}
