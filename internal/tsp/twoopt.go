package tsp

import "slices"

const epsilon = 1e-9

func twoOptRefine(route []int, matrix [][]float64) []int {
	n := len(route)
	if n < 4 {
		return slices.Clone(route)
	}

	result := slices.Clone(route)
	improved := true

	for improved {
		improved = false

		for i := range n - 2 {
			for j := i + 2; j < n-1; j++ {
				a := result[i]
				b := result[i+1]
				c := result[j]
				d := result[j+1]

				if delta := matrix[a][c] + matrix[b][d] - matrix[a][b] - matrix[c][d]; delta < -epsilon {
					slices.Reverse(result[i+1 : j+1])

					improved = true
				}
			}
		}
	}

	return result
}
