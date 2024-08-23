package manager

import (
	"reflect"
	"testing"
)

func TestSortIndexers(t *testing.T) {
	tests := []struct {
		name string
		in   []Indexer
		out  []Indexer
	}{
		{
			name: "already sorted",
			in:   []Indexer{{Priority: 1}, {Priority: 2}, {Priority: 3}},
			out:  []Indexer{{Priority: 1}, {Priority: 2}, {Priority: 3}},
		},
		{
			name: "reversed",
			in:   []Indexer{{Priority: 100}, {Priority: 3}, {Priority: 0}},
			out:  []Indexer{{Priority: 0}, {Priority: 3}, {Priority: 100}},
		},
		{
			name: "negative and unsorted",
			in:   []Indexer{{Priority: 33}, {Priority: -1}, {Priority: 23}, {Priority: 10}, {Priority: 32}},
			out:  []Indexer{{Priority: -1}, {Priority: 10}, {Priority: 23}, {Priority: 32}, {Priority: 33}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortIndexers(tt.in)
			if !reflect.DeepEqual(tt.out, tt.in) {
				t.Errorf("wanted: %v; got %v", tt.out, tt.in)
			}
		})
	}
}
