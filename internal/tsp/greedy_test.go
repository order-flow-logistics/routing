package tsp

import (
	"slices"
	"testing"
)

func TestGreedyNearestNeighbor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		matrix [][]float64
		start  int
		want   []int
	}{
		{"line from left end", lineMatrix(4), 0, []int{0, 1, 2, 3}},
		{"line from interior keeps nearest-first", lineMatrix(4), 2, []int{2, 1, 0, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := greedyNearestNeighbor(tt.matrix, tt.start); !slices.Equal(got, tt.want) {
				t.Errorf("greedyNearestNeighbor(%d) = %v, want %v", tt.start, got, tt.want)
			}
		})
	}
}
