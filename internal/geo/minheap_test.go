package geo

import (
	"math"
	"testing"
)

func TestMinHeap_PopsInAscendingDistance(t *testing.T) {
	t.Parallel()

	dists := []float64{5, 1, 4, 1, 3, 2, 0, 9, 7}

	var h minHeap
	for i, d := range dists {
		h.push(heapItem{id: int64(i), dist: d})
	}

	if h.len() != len(dists) {
		t.Fatalf("len = %d, want %d", h.len(), len(dists))
	}

	prev := math.Inf(-1)
	popped := 0

	for {
		it, ok := h.pop()
		if !ok {
			break
		}

		if it.dist < prev {
			t.Errorf("pop out of order: got %g after %g", it.dist, prev)
		}

		prev = it.dist
		popped++
	}

	if popped != len(dists) {
		t.Errorf("popped %d items, want %d", popped, len(dists))
	}
}

func TestMinHeap_PopEmpty(t *testing.T) {
	t.Parallel()

	var h minHeap
	if _, ok := h.pop(); ok {
		t.Error("pop on empty heap returned ok=true")
	}
}
