package geo

import (
	"math"
	"slices"
	"testing"
)

func testGraph() *Graph {
	return &Graph{
		Nodes: []Node{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}},
		Adj: map[int64][]Edge{
			1: {{To: 2, Weight: 1}, {To: 3, Weight: 4}},
			2: {{To: 3, Weight: 1}, {To: 4, Weight: 5}},
			3: {{To: 4, Weight: 1}},
			4: {},
			5: {},
		},
	}
}

func TestDijkstra(t *testing.T) {
	t.Parallel()

	sp := dijkstra(testGraph(), 1)

	wantDist := map[int64]float64{1: 0, 2: 1, 3: 2, 4: 3, 5: math.Inf(1)}
	for id, want := range wantDist {
		if got := sp.dist[id]; got != want {
			t.Errorf("dist[%d] = %v, want %v", id, got, want)
		}
	}
}

func TestReconstructPath(t *testing.T) {
	t.Parallel()

	sp := dijkstra(testGraph(), 1)

	tests := []struct {
		name   string
		target int64
		want   []int64
	}{
		{"source only", 1, []int64{1}},
		{"direct neighbor", 2, []int64{1, 2}},
		{"cheaper via intermediate", 3, []int64{1, 2, 3}},
		{"longest chain", 4, []int64{1, 2, 3, 4}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := reconstructPath(sp.prev, tt.target); !slices.Equal(got, tt.want) {
				t.Errorf("reconstructPath(%d) = %v, want %v", tt.target, got, tt.want)
			}
		})
	}
}
