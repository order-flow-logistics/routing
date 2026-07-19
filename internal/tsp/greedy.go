package tsp

import "math"

func greedyNearestNeighbor(matrix [][]float64, start int) []int {
	n := len(matrix)
	visited := make([]bool, n)
	route := []int{start}
	visited[start] = true
	current := start

	for step := 1; step < n; step++ {
		best := -1
		bestDist := math.Inf(1)

		for v := range n {
			if visited[v] {
				continue
			}

			if matrix[current][v] < bestDist {
				bestDist = matrix[current][v]
				best = v
			}
		}

		if best == -1 {
			break
		}

		route = append(route, best)
		visited[best] = true
		current = best
	}

	return route
}
