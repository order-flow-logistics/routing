// Package tsp solves the travelling-salesman visiting order for a distance
// matrix: exact Held-Karp for small n, greedy nearest-neighbour + 2-opt above it.
package tsp

// HeldKarpLimit is the largest n for which exact Held-Karp runs (TS parity).
const HeldKarpLimit = 12

// Solver names the algorithm that produced a route, matching the TS strings.
type Solver string

// Solver values, matching the TS solver strings exactly.
const (
	SolverGreedy       Solver = "greedy"
	SolverHeldKarp     Solver = "held-karp"
	SolverGreedyTwoOpt Solver = "greedy+2opt"
)

// Result is the solved tour; GreedyDistance is set only for the greedy+2opt path.
type Result struct {
	Route          []int
	TotalDistance  float64
	Solver         Solver
	GreedyDistance *float64
}

// Solve returns the tour from start: exact for n <= HeldKarpLimit, else greedy +
// 2-opt. Mirrors TS solveTSP.
func Solve(matrix [][]float64, start int) Result {
	n := len(matrix)
	if n <= 1 {
		return Result{Route: []int{start}, Solver: SolverGreedy}
	}

	if n <= HeldKarpLimit {
		route := heldKarpExact(matrix, start)

		return Result{
			Route:         route,
			TotalDistance: routeLength(route, matrix),
			Solver:        SolverHeldKarp,
		}
	}

	greedy := greedyNearestNeighbor(matrix, start)
	greedyDist := routeLength(greedy, matrix)
	refined := twoOptRefine(greedy, matrix)

	return Result{
		Route:          refined,
		TotalDistance:  routeLength(refined, matrix),
		Solver:         SolverGreedyTwoOpt,
		GreedyDistance: &greedyDist,
	}
}

func routeLength(route []int, matrix [][]float64) float64 {
	sum := 0.0
	for i := 0; i+1 < len(route); i++ {
		sum += matrix[route[i]][route[i+1]]
	}

	return sum
}
