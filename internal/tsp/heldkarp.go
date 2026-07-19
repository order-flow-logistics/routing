package tsp

import (
	"math"
	"slices"
)

func heldKarpExact(matrix [][]float64, start int) []int {
	n := len(matrix)
	if n == 1 {
		return []int{start}
	}

	dp, parent := heldKarpTable(matrix, start)

	return heldKarpPath(dp, parent, n, start)
}

func heldKarpTable(matrix [][]float64, start int) (dp []float64, parent []int) {
	n := len(matrix)
	size := 1 << n

	dp = make([]float64, size*n)
	for i := range dp {
		dp[i] = math.Inf(1)
	}

	parent = make([]int, size*n)
	for i := range parent {
		parent[i] = -1
	}

	dp[(1<<start)*n+start] = 0

	for mask := range size {
		if mask&(1<<start) == 0 {
			continue
		}

		for last := range n {
			base := dp[mask*n+last]
			if mask&(1<<last) == 0 || math.IsInf(base, 1) {
				continue
			}

			for next := range n {
				if mask&(1<<next) != 0 {
					continue
				}

				newMask := mask | (1 << next)
				if cand := base + matrix[last][next]; cand < dp[newMask*n+next] {
					dp[newMask*n+next] = cand
					parent[newMask*n+next] = last
				}
			}
		}
	}

	return dp, parent
}

func heldKarpPath(dp []float64, parent []int, n, start int) []int {
	full := (1 << n) - 1
	bestEnd := -1
	bestCost := math.Inf(1)

	for i := range n {
		if i == start {
			continue
		}

		if dp[full*n+i] < bestCost {
			bestCost = dp[full*n+i]
			bestEnd = i
		}
	}

	if bestEnd == -1 {
		return []int{start}
	}

	path := make([]int, 0, n)
	mask := full

	for curr := bestEnd; curr != -1; {
		path = append(path, curr)
		prev := parent[mask*n+curr]
		mask ^= 1 << curr
		curr = prev
	}

	slices.Reverse(path)

	return path
}
