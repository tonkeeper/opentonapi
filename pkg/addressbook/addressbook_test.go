package addressbook

import (
	"reflect"
	"testing"
)

func Test_unique(t *testing.T) {
	tests := []struct {
		name      string
		approvers []string
		want      []string
	}{
		{
			name:      "all good",
			approvers: []string{"tonkeeper", "getgems", "getgems", "tonkeeper"},
			want:      []string{"getgems", "tonkeeper"},
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
