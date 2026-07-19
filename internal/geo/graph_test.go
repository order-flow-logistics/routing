package geo

import (
	"math"
	"slices"
	"testing"
)

func neighbors(edges []Edge) []int64 {
	to := make([]int64, len(edges))
	for i, e := range edges {
		to[i] = e.To
	}

	return to
}

func findEdge(edges []Edge, to int64) (Edge, bool) {
	for _, e := range edges {
		if e.To == to {
			return e, true
		}
	}

	return Edge{}, false
}

func TestBuildGraph(t *testing.T) {
	t.Parallel()

	nodes := []OSMNode{
		{ID: 1, Lat: 0, Lng: 0},
		{ID: 2, Lat: 0, Lng: 1},
		{ID: 3, Lat: 0, Lng: 2},
	}
	ways := []OSMWay{
		{ID: 10, Nodes: []int64{1, 2}, Oneway: false},
		{ID: 11, Nodes: []int64{2, 3}, Oneway: true},
		{ID: 12, Nodes: []int64{1, 99}, Oneway: false}, // 99 unknown -> pair skipped
	}

	g := BuildGraph(nodes, ways)

	if len(g.Nodes) != len(nodes) {
		t.Fatalf("Nodes length = %d, want %d", len(g.Nodes), len(nodes))
	}

	tests := []struct {
		name   string
		source int64
		wantTo []int64
	}{
		{"two-way forward edge", 1, []int64{2}},
		{"two-way reverse plus one-way forward", 2, []int64{1, 3}},
		{"one-way has no reverse edge", 3, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := neighbors(g.Adj[tt.source]); !slices.Equal(got, tt.wantTo) {
				t.Errorf("Adj[%d] neighbors = %v, want %v", tt.source, got, tt.wantTo)
			}
		})
	}

	t.Run("edge weight is haversine distance", func(t *testing.T) {
		t.Parallel()

		want := HaversineKm(LatLng{Lat: 0, Lng: 0}, LatLng{Lat: 0, Lng: 1})

		e, ok := findEdge(g.Adj[1], 2)
		if !ok {
			t.Fatal("expected edge 1->2")
		}

		if math.Abs(e.Weight-want) > distEpsilon {
			t.Errorf("weight = %.12f, want %.12f", e.Weight, want)
		}
	})
}

func TestFindNearestNode(t *testing.T) {
	t.Parallel()

	nodes := []Node{
		{ID: 1, LatLng: LatLng{Lat: 0, Lng: 0}},
		{ID: 2, LatLng: LatLng{Lat: 0, Lng: 1}},
		{ID: 3, LatLng: LatLng{Lat: 0, Lng: 2}},
	}

	tests := []struct {
		name   string
		target LatLng
		wantID int64
	}{
		{"closest to first", LatLng{Lat: 0, Lng: 0.1}, 1},
		{"closest to middle", LatLng{Lat: 0, Lng: 0.9}, 2},
		{"exact match", LatLng{Lat: 0, Lng: 2}, 3},
		{"equidistant keeps earliest", LatLng{Lat: 0, Lng: 0.5}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := FindNearestNode(nodes, tt.target).ID; got != tt.wantID {
				t.Errorf("FindNearestNode(%+v).ID = %d, want %d", tt.target, got, tt.wantID)
			}
		})
	}
}
