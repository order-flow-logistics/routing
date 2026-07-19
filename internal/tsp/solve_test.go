package tsp

import (
	"math"
	"slices"
	"testing"
)

func lineMatrix(n int) [][]float64 {
	m := make([][]float64, n)
	for i := range m {
		m[i] = make([]float64, n)
		for j := range n {
			m[i][j] = math.Abs(float64(i - j))
		}
	}

	return m
}

func symmetricMatrix(data []byte) (int, [][]float64) {
	n := int(data[0]%6) + 2
	m := make([][]float64, n)

	for i := range m {
		m[i] = make([]float64, n)
	}

	k := 1

	for i := range n {
		for j := i + 1; j < n; j++ {
			w := float64(data[k%len(data)]) + 1
			m[i][j] = w
			m[j][i] = w
			k++
		}
	}

	return n, m
}

func assertTour(t *testing.T, route []int, n, start int) {
	t.Helper()

	if len(route) != n {
		t.Fatalf("route %v has length %d, want %d", route, len(route), n)
	}

	seen := make([]bool, n)

	for idx, v := range route {
		if idx == 0 && v != start {
			t.Fatalf("route[0] = %d, want start %d", v, start)
		}

		if v < 0 || v >= n || seen[v] {
			t.Fatalf("route %v is not a permutation of 0..%d", route, n-1)
		}

		seen[v] = true
	}
}

func TestSolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		matrix     [][]float64
		start      int
		wantRoute  []int
		wantSolver Solver
		wantTotal  float64
	}{
		{"single node", [][]float64{{0}}, 0, []int{0}, SolverGreedy, 0},
		{"pair", [][]float64{{0, 5}, {5, 0}}, 0, []int{0, 1}, SolverHeldKarp, 5},
		{"line of four", lineMatrix(4), 0, []int{0, 1, 2, 3}, SolverHeldKarp, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Solve(tt.matrix, tt.start)

			if !slices.Equal(got.Route, tt.wantRoute) {
				t.Errorf("Route = %v, want %v", got.Route, tt.wantRoute)
			}

			if got.Solver != tt.wantSolver {
				t.Errorf("Solver = %q, want %q", got.Solver, tt.wantSolver)
			}

			if math.Abs(got.TotalDistance-tt.wantTotal) > epsilon {
				t.Errorf("TotalDistance = %v, want %v", got.TotalDistance, tt.wantTotal)
			}
		})
	}
}

func FuzzSolve(f *testing.F) {
	f.Add([]byte{3, 5, 8, 2, 9, 1, 7, 4, 6})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 4 {
			return
		}

		n, m := symmetricMatrix(data)

		res := Solve(m, 0)

		assertTour(t, res.Route, n, 0)

		if math.IsInf(res.TotalDistance, 0) || res.TotalDistance < 0 {
			t.Fatalf("TotalDistance = %v, want finite non-negative", res.TotalDistance)
		}
	})
}
