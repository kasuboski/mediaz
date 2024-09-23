package manager

import (
	"fmt"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
)

func TestQualitySizeCutoff(t *testing.T) {
	tests := []struct {
		name       string
		size       uint64
		runtime    uint64
		want       bool
		definition storage.QualityDefinition
	}{
		{
			name:    "does not meet minimum size",
			size:    1000,
			runtime: 60,
			definition: storage.QualityDefinition{
				MinSize:       17,
				MaxSize:       2000,
				PreferredSize: 1999,
			},
			want: false,
		},
		{
			name:    "meets criteria",
			size:    1026,
			runtime: 60,
			definition: storage.QualityDefinition{
				MinSize:       17.0,
				MaxSize:       2000,
				PreferredSize: 1999,
			},
			want: true,
		},

		{
			name:    "ratio too big",
			size:    120_001,
			runtime: 60,
			definition: storage.QualityDefinition{
				MinSize:       17.0,
				MaxSize:       2000,
				PreferredSize: 1999,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d,%d", tt.size, tt.runtime), func(t *testing.T) {
			if got := MeetsQualitySize(tt.definition, tt.size, tt.runtime); got != tt.want {
				t.Errorf("got %v; want %v", got, tt.want)
			}
		})
	}
}
