package geo

import (
	"math"
	"runtime"
	"sync"
)

// Matrix is an all-pairs result: Dist[i][j] in km, Paths[i][j] the node id chain
// (nil Paths when not requested; an empty inner slice when j is unreachable).
type Matrix struct {
	Dist  [][]float64
	Paths [][][]int64
}

// ComputeMatrix runs one Dijkstra per waypoint concurrently over disjoint rows,
// mirroring TS computeOsmDistanceMatrix. concurrency <= 0 means runtime.NumCPU().
func ComputeMatrix(g *Graph, waypointIDs []int64, wantPaths bool, concurrency int) *Matrix {
	n := len(waypointIDs)

	m := &Matrix{Dist: make([][]float64, n)}
	if wantPaths {
		m.Paths = make([][][]int64, n)
	}

	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
	}

	var wg sync.WaitGroup

	sem := make(chan struct{}, concurrency)

	for i := range waypointIDs {
		wg.Add(1)

		sem <- struct{}{}

		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			dist, paths := singleSourceRow(g, waypointIDs, i, wantPaths)

			m.Dist[i] = dist
			if wantPaths {
				m.Paths[i] = paths
			}
		}()
	}

	wg.Wait()

	return m
}

func singleSourceRow(g *Graph, waypointIDs []int64, i int, wantPaths bool) ([]float64, [][]int64) {
	n := len(waypointIDs)
	sp := dijkstra(g, waypointIDs[i])
	dist := make([]float64, n)

	var paths [][]int64
	if wantPaths {
		paths = make([][]int64, n)
	}

	for j := range waypointIDs {
		if i == j {
			if wantPaths {
				paths[j] = []int64{waypointIDs[i]}
			}

			continue
		}

		d, ok := sp.dist[waypointIDs[j]]
		if !ok {
			d = math.Inf(1)
		}

		dist[j] = d
		if wantPaths {
			if math.IsInf(d, 1) {
				paths[j] = []int64{}
			} else {
				paths[j] = reconstructPath(sp.prev, waypointIDs[j])
			}
		}
	}

	return dist, paths
}

// HaversineMatrix is the straight-line fallback when no road graph is available;
// symmetric, diagonal zero, no paths. Mirrors TS buildHaversineMatrix.
func HaversineMatrix(waypoints []LatLng) *Matrix {
	n := len(waypoints)
	dist := make([][]float64, n)

	for i := range dist {
		dist[i] = make([]float64, n)
	}

	for i := range n {
		for j := i + 1; j < n; j++ {
			d := HaversineKm(waypoints[i], waypoints[j])
			dist[i][j] = d
			dist[j][i] = d
		}
	}

	return &Matrix{Dist: dist}
}
