package geo

type heapItem struct {
	id   int64
	dist float64
}

type minHeap struct {
	items []heapItem
}

func (h *minHeap) len() int { return len(h.items) }

func (h *minHeap) push(it heapItem) {
	h.items = append(h.items, it)
	h.bubbleUp(len(h.items) - 1)
}

func (h *minHeap) pop() (heapItem, bool) {
	if len(h.items) == 0 {
		return heapItem{}, false
	}

	top := h.items[0]
	last := h.items[len(h.items)-1]
	h.items = h.items[:len(h.items)-1]

	if len(h.items) > 0 {
		h.items[0] = last
		h.siftDown(0)
	}

	return top, true
}

func (h *minHeap) bubbleUp(i int) {
	for i > 0 {
		parent := (i - 1) >> 1
		if h.items[parent].dist <= h.items[i].dist {
			break
		}

		h.items[parent], h.items[i] = h.items[i], h.items[parent]
		i = parent
	}
}

func (h *minHeap) siftDown(i int) {
	n := len(h.items)

	for {
		smallest := i
		left := 2*i + 1
		right := 2*i + 2

		if left < n && h.items[left].dist < h.items[smallest].dist {
			smallest = left
		}

		if right < n && h.items[right].dist < h.items[smallest].dist {
			smallest = right
		}

		if smallest == i {
			break
		}

		h.items[smallest], h.items[i] = h.items[i], h.items[smallest]
		i = smallest
	}
}
