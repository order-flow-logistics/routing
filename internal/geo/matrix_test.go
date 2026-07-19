package geo

import (
	"math"
	"reflect"
	"slices"
	"testing"
)

func TestComputeMatrix(t *testing.T) {
	t.Parallel()

	g := testGraph()
	wp := []int64{1, 2, 3, 4}

	m := ComputeMatrix(g, wp, true, 1)

	wantDist := [][]float64{
		{0, 1, 2, 3},
		{math.Inf(1), 0, 1, 2},
		{math.Inf(1), math.Inf(1), 0, 1},
		{math.Inf(1), math.Inf(1), math.Inf(1), 0},
	}
	if !reflect.DeepEqual(m.Dist, wantDist) {
		t.Errorf("Dist = %v, want %v", m.Dist, wantDist)
	}

	if got := m.Paths[0][3]; !slices.Equal(got, []int64{1, 2, 3, 4}) {
		t.Errorf("Paths[0][3] = %v, want [1 2 3 4]", got)
	}

	if got := m.Paths[2][2]; !slices.Equal(got, []int64{3}) {
		t.Errorf("Paths[2][2] = %v, want [3]", got)
	}

	if got := m.Paths[3][0]; len(got) != 0 {
		t.Errorf("unreachable Paths[3][0] = %v, want empty", got)
	}
}

func TestComputeMatrix_WithoutPaths(t *testing.T) {
	t.Parallel()

	m := ComputeMatrix(testGraph(), []int64{1, 2, 3, 4}, false, 1)
	if m.Paths != nil {
		t.Errorf("Paths = %v, want nil", m.Paths)
	}
}

func TestComputeMatrix_DeterministicAcrossConcurrency(t *testing.T) {
	t.Parallel()

	g := testGraph()
	wp := []int64{1, 2, 3, 4}

	serial := ComputeMatrix(g, wp, true, 1)
	parallel := ComputeMatrix(g, wp, true, 8)

	if !reflect.DeepEqual(serial, parallel) {
		t.Errorf("result differs between concurrency 1 and 8:\n serial=%v\n parallel=%v", serial, parallel)
	}
}

func TestHaversineMatrix(t *testing.T) {
	t.Parallel()

	wp := []LatLng{
		{Lat: 0, Lng: 0},
		{Lat: 0, Lng: 1},
		{Lat: 1, Lng: 0},
	}

	m := HaversineMatrix(wp)

	for i := range wp {
		if m.Dist[i][i] != 0 {
			t.Errorf("diagonal Dist[%d][%d] = %v, want 0", i, i, m.Dist[i][i])
		}

		for j := range wp {
			if math.Abs(m.Dist[i][j]-m.Dist[j][i]) > distEpsilon {
				t.Errorf("not symmetric at [%d][%d]", i, j)
			}
		}
	}

	if m.Paths != nil {
		t.Errorf("Paths = %v, want nil", m.Paths)
	}
}
