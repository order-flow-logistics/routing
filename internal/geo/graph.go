package geo

// Node is a graph vertex: an OSM node id plus its coordinate.
type Node struct {
	ID int64
	LatLng
}

// Edge is a directed, weighted (km) adjacency entry.
type Edge struct {
	To     int64
	Weight float64
}

// Graph is the road network; built once, then read concurrently without mutation.
type Graph struct {
	Nodes []Node
	Adj   map[int64][]Edge
}

// OSMNode is a raw Overpass node.
type OSMNode struct {
	ID  int64
	Lat float64
	Lng float64
}

// OSMWay is a raw Overpass way; Oneway drops the reverse edges.
type OSMWay struct {
	ID     int64
	Nodes  []int64
	Oneway bool
}

// BuildGraph mirrors TS buildOsmGraph; edge order follows way order (Dijkstra tie-breaks).
func BuildGraph(osmNodes []OSMNode, osmWays []OSMWay) *Graph {
	nodeByID := make(map[int64]OSMNode, len(osmNodes))
	for _, n := range osmNodes {
		nodeByID[n.ID] = n
	}

	nodes := make([]Node, len(osmNodes))
	adj := make(map[int64][]Edge, len(osmNodes))

	for i, n := range osmNodes {
		nodes[i] = Node{ID: n.ID, LatLng: LatLng{Lat: n.Lat, Lng: n.Lng}}
		adj[n.ID] = nil
	}

	for _, way := range osmWays {
		for i := 0; i+1 < len(way.Nodes); i++ {
			aID, bID := way.Nodes[i], way.Nodes[i+1]

			a, aOK := nodeByID[aID]
			b, bOK := nodeByID[bID]

			if !aOK || !bOK {
				continue
			}

			w := HaversineKm(LatLng{Lat: a.Lat, Lng: a.Lng}, LatLng{Lat: b.Lat, Lng: b.Lng})
			adj[aID] = append(adj[aID], Edge{To: bID, Weight: w})

			if !way.Oneway {
				adj[bID] = append(adj[bID], Edge{To: aID, Weight: w})
			}
		}
	}

	return &Graph{Nodes: nodes, Adj: adj}
}

// FindNearestNode keeps the earliest node on ties, matching TS. Needs len(nodes) > 0.
func FindNearestNode(nodes []Node, target LatLng) Node {
	nearest := nodes[0]
	minDist := HaversineKm(nearest.LatLng, target)

	for _, n := range nodes {
		if d := HaversineKm(n.LatLng, target); d < minDist {
			minDist = d
			nearest = n
		}
	}

	return nearest
}
