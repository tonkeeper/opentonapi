package bath

import (
	"testing"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/references"
)

func TestLiquidStakingImplementation(t *testing.T) {
	if got := liquidStakingImplementation(references.TonstakersAccountPool); got != core.StakingImplementationLiquidTF {
		t.Errorf("Tonstakers pool: got %v", got)
	}
	if got := liquidStakingImplementation(references.KtonAccountPool); got != core.StakingImplementationKton {
		t.Errorf("KTON pool: got %v", got)
	}
}
