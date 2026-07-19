package geo

import (
	"math"
	"slices"
)

type shortestPaths struct {
	dist map[int64]float64
	prev map[int64]int64
}

func dijkstra(g *Graph, sourceID int64) shortestPaths {
	dist := make(map[int64]float64, len(g.Nodes))
	for _, n := range g.Nodes {
		dist[n.ID] = math.Inf(1)
	}

	dist[sourceID] = 0
	prev := make(map[int64]int64)

	var pq minHeap

	pq.push(heapItem{id: sourceID, dist: 0})

	for pq.len() > 0 {
		item, _ := pq.pop()

		u := item.id
		if item.dist > dist[u] {
			continue
		}

		for _, e := range g.Adj[u] {
			if alt := dist[u] + e.Weight; alt < dist[e.To] {
				dist[e.To] = alt
				prev[e.To] = u
				pq.push(heapItem{id: e.To, dist: alt})
			}
		}
	}

	return shortestPaths{dist: dist, prev: prev}
}

func reconstructPath(prev map[int64]int64, targetID int64) []int64 {
	path := []int64{targetID}

	for {
		p, ok := prev[path[len(path)-1]]
		if !ok {
			break
		}

		path = append(path, p)
	}

	slices.Reverse(path)

	return path
}
