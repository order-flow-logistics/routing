package tsp

import (
	"encoding/json"
	"math"
	"os"
	"slices"
	"testing"
)

func loadGolden(t *testing.T, path string, dst any) {
	t.Helper()

	data, err := os.ReadFile(path) //nolint:gosec // reads a trusted, in-repo testdata fixture path
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
}

type solveGolden struct {
	Name          string      `json:"name"`
	Matrix        [][]float64 `json:"matrix"`
	Start         int         `json:"start"`
	Route         []int       `json:"route"`
	TotalDistance float64     `json:"totalDistance"`
	Solver        string      `json:"solver"`
}

func TestSolve_Golden(t *testing.T) {
	t.Parallel()

	var cases []solveGolden

	loadGolden(t, "testdata/solve.json", &cases)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := Solve(tc.Matrix, tc.Start)

			if !slices.Equal(got.Route, tc.Route) {
				t.Errorf("route = %v, want %v (TS)", got.Route, tc.Route)
			}

			if string(got.Solver) != tc.Solver {
				t.Errorf("solver = %q, want %q (TS)", got.Solver, tc.Solver)
			}

			if math.Abs(got.TotalDistance-tc.TotalDistance) > epsilon {
				t.Errorf("totalDistance = %v, want %v (TS)", got.TotalDistance, tc.TotalDistance)
			}
		})
	}
}
