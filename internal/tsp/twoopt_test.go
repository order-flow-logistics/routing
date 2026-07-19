package tsp

import (
	"math"
	"slices"
	"testing"
)

func TestTwoOptRefine(t *testing.T) {
	t.Parallel()

	t.Run("uncrosses a crossing route", func(t *testing.T) {
		t.Parallel()

		m := lineMatrix(4)

		got := twoOptRefine([]int{0, 2, 1, 3}, m)
		if l := routeLength(got, m); math.Abs(l-3) > epsilon {
			t.Errorf("refined length = %v (route %v), want 3", l, got)
		}
	})

	t.Run("route shorter than four is returned unchanged", func(t *testing.T) {
		t.Parallel()

		in := []int{0, 2, 1}

		if got := twoOptRefine(in, lineMatrix(3)); !slices.Equal(got, in) {
			t.Errorf("got %v, want unchanged %v", got, in)
		}
	})
}
