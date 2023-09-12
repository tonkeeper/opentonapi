package addressbook

import (
	"reflect"
	"testing"

	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func Test_unique(t *testing.T) {
	tests := []struct {
		name      string
		approvers []oas.NftItemApprovedByItem
		want      []oas.NftItemApprovedByItem
	}{
		{
			name:      "all good",
			approvers: []oas.NftItemApprovedByItem{oas.NftItemApprovedByItemTonkeeper, oas.NftItemApprovedByItemGetgems, oas.NftItemApprovedByItemGetgems, oas.NftItemApprovedByItemTonkeeper},
			want:      []oas.NftItemApprovedByItem{oas.NftItemApprovedByItemGetgems, oas.NftItemApprovedByItemTonkeeper},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newList := unique(tt.approvers)
			if !reflect.DeepEqual(newList, tt.want) {
				t.Errorf("unique() = %v, want %v", newList, tt.want)
			}
		})
	}
}
