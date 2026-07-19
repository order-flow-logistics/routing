package tsp

import (
	"math"
	"testing"
)

func permute(a []int, k int, visit func([]int)) {
	if k == len(a) {
		visit(a)

		return
	}

	for i := k; i < len(a); i++ {
		a[k], a[i] = a[i], a[k]
		permute(a, k+1, visit)
		a[k], a[i] = a[i], a[k]
	}
}

func bruteForceLength(matrix [][]float64, start int) float64 {
	var others []int

	for i := range matrix {
		if i != start {
			others = append(others, i)
		}
	}

	best := math.Inf(1)

	permute(others, 0, func(p []int) {
		route := append([]int{start}, p...)
		if l := routeLength(route, matrix); l < best {
			best = l
		}
	})

	return best
}

func TestHeldKarpExact_MatchesBruteForce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		matrix [][]float64
		start  int
	}{
		{"line of five", lineMatrix(5), 0},
		{"line from interior start", lineMatrix(5), 3},
		{"asymmetric costs", [][]float64{{0, 2, 9, 10}, {1, 0, 6, 4}, {15, 7, 0, 8}, {6, 3, 12, 0}}, 0},
		{"all equal costs", [][]float64{{0, 1, 1, 1}, {1, 0, 1, 1}, {1, 1, 0, 1}, {1, 1, 1, 0}}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			route := heldKarpExact(tt.matrix, tt.start)

			assertTour(t, route, len(tt.matrix), tt.start)

			got := routeLength(route, tt.matrix)

			if want := bruteForceLength(tt.matrix, tt.start); math.Abs(got-want) > epsilon {
				t.Errorf("heldKarp length = %v, brute-force optimal = %v", got, want)
			}
		})
	}
}
