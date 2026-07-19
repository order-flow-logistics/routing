package geo

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
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

type haversineGolden struct {
	Name string  `json:"name"`
	A    LatLng  `json:"a"`
	B    LatLng  `json:"b"`
	Km   float64 `json:"km"`
}

func TestHaversineKm_Golden(t *testing.T) {
	t.Parallel()

	var cases []haversineGolden

	loadGolden(t, "testdata/haversine.json", &cases)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			if got := HaversineKm(tc.A, tc.B); math.Abs(got-tc.Km) > distEpsilon {
				t.Errorf("HaversineKm = %.12f, want %.12f (TS)", got, tc.Km)
			}
		})
	}
}

type matrixGolden struct {
	Name        string      `json:"name"`
	Nodes       []OSMNode   `json:"nodes"`
	Ways        []OSMWay    `json:"ways"`
	WaypointIDs []int64     `json:"waypointIds"`
	Distances   [][]float64 `json:"distances"`
	Paths       [][][]int64 `json:"paths"`
}

func TestComputeMatrix_Golden(t *testing.T) {
	t.Parallel()

	var cases []matrixGolden

	loadGolden(t, "testdata/matrix.json", &cases)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			g := BuildGraph(tc.Nodes, tc.Ways)
			m := ComputeMatrix(g, tc.WaypointIDs, true, 1)

			for i := range tc.Distances {
				for j := range tc.Distances[i] {
					if math.Abs(m.Dist[i][j]-tc.Distances[i][j]) > distEpsilon {
						t.Errorf("Dist[%d][%d] = %.12f, want %.12f (TS)", i, j, m.Dist[i][j], tc.Distances[i][j])
					}
				}
			}

			if !reflect.DeepEqual(m.Paths, tc.Paths) {
				t.Errorf("paths differ from TS:\n got  %v\n want %v", m.Paths, tc.Paths)
			}
		})
	}
}
