package routing

import (
	"context"
	"log/slog"
	"runtime"
	"sync"

	"github.com/order-flow-logistics/routing/internal/geo"
	"github.com/order-flow-logistics/routing/internal/tsp"
)

// Service orchestrates one compute call: shared graph fetch, then per-group
// matrix, TSP, geometry, and durations.
type Service struct {
	net         RoadNetworkProvider
	dur         DurationProvider
	log         *slog.Logger
	concurrency int
}

// Option configures a Service.
type Option func(*Service)

// WithConcurrency bounds parallel group and matrix work; non-positive is ignored.
func WithConcurrency(n int) Option {
	return func(s *Service) {
		if n > 0 {
			s.concurrency = n
		}
	}
}

// WithLogger sets the request logger; nil is ignored.
func WithLogger(l *slog.Logger) Option {
	return func(s *Service) {
		if l != nil {
			s.log = l
		}
	}
}

// New builds a Service from its required ports plus optional tuning.
func New(net RoadNetworkProvider, dur DurationProvider, opts ...Option) *Service {
	s := &Service{
		net:         net,
		dur:         dur,
		log:         slog.Default(),
		concurrency: runtime.NumCPU(),
	}

	for _, o := range opts {
		o(s)
	}

	return s
}

// ComputeRoutes solves every group concurrently over a single shared road graph.
func (s *Service) ComputeRoutes(ctx context.Context, req Request) (*Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	graph := s.fetchGraph(ctx, req.Groups)

	var coordByID map[int64]geo.LatLng
	if graph != nil {
		coordByID = coordMap(graph)
	}

	routes := make([]ComputedRoute, len(req.Groups))

	var wg sync.WaitGroup

	sem := make(chan struct{}, s.concurrency)

	for i := range req.Groups {
		wg.Add(1)

		sem <- struct{}{}

		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			routes[i] = s.computeGroup(ctx, req.Groups[i], graph, coordByID, req.IncludeGeometry, req.IncludeDurations)
		}()
	}

	wg.Wait()

	return &Response{Routes: routes}, nil
}

func (s *Service) fetchGraph(ctx context.Context, groups []Group) *geo.Graph {
	var points []geo.LatLng

	for _, g := range groups {
		for _, wp := range g.Waypoints {
			points = append(points, wp.Location)
		}
	}

	graph, err := s.net.FetchGraph(ctx, points)
	if err != nil {
		s.log.WarnContext(ctx, "road network unavailable; using haversine fallback", "error", err)

		return nil
	}

	return graph
}

func (s *Service) computeGroup(ctx context.Context, g Group, graph *geo.Graph, coordByID map[int64]geo.LatLng, wantGeometry, wantDurations bool) ComputedRoute {
	if len(g.Waypoints) == 0 {
		return ComputedRoute{GroupID: g.ID}
	}

	matrix, source := s.buildMatrix(g, graph, wantGeometry)
	sol := tsp.Solve(matrix.Dist, 0)
	ordered := orderWaypoints(g, sol, matrix)

	route := ComputedRoute{
		GroupID:        g.ID,
		Waypoints:      ordered,
		Solver:         sol.Solver,
		DistanceSource: source,
	}

	for _, ow := range ordered {
		route.TotalDistanceKm += ow.DistanceFromPrevKm
	}

	if wantGeometry && matrix.Paths != nil {
		route.Geometry = buildGeometry(sol.Route, matrix, coordByID)
	}

	if wantDurations {
		s.attachDurations(ctx, route.Waypoints)
	}

	return route
}

func (s *Service) buildMatrix(g Group, graph *geo.Graph, wantGeometry bool) (*geo.Matrix, DistanceSource) {
	if graph != nil && len(graph.Nodes) > 0 {
		ids := make([]int64, len(g.Waypoints))
		for i, wp := range g.Waypoints {
			ids[i] = geo.FindNearestNode(graph.Nodes, wp.Location).ID
		}

		return geo.ComputeMatrix(graph, ids, wantGeometry, s.concurrency), DistanceSourceOSM
	}

	locs := make([]geo.LatLng, len(g.Waypoints))
	for i, wp := range g.Waypoints {
		locs[i] = wp.Location
	}

	return geo.HaversineMatrix(locs), DistanceSourceHaversine
}

func (s *Service) attachDurations(ctx context.Context, ordered []OrderedWaypoint) {
	for i := 1; i < len(ordered); i++ {
		d, ok, err := s.dur.LegDuration(ctx, ordered[i-1].Location, ordered[i].Location)
		if err != nil {
			s.log.WarnContext(ctx, "leg duration failed", "error", err, "leg", i)

			continue
		}

		if ok {
			ordered[i].DurationFromPrev = d
			ordered[i].HasDuration = true
		}
	}
}

func orderWaypoints(g Group, sol tsp.Result, matrix *geo.Matrix) []OrderedWaypoint {
	ordered := make([]OrderedWaypoint, len(sol.Route))

	for pos, idx := range sol.Route {
		ow := OrderedWaypoint{Waypoint: g.Waypoints[idx]}
		if pos > 0 {
			ow.DistanceFromPrevKm = matrix.Dist[sol.Route[pos-1]][idx]
		}

		ordered[pos] = ow
	}

	return ordered
}

func buildGeometry(route []int, matrix *geo.Matrix, coordByID map[int64]geo.LatLng) []Polyline {
	var segments []Polyline

	for i := 0; i+1 < len(route); i++ {
		path := matrix.Paths[route[i]][route[i+1]]
		pts := make([]geo.LatLng, 0, len(path))

		for _, nodeID := range path {
			if c, ok := coordByID[nodeID]; ok {
				pts = append(pts, c)
			}
		}

		if len(pts) > 1 {
			segments = append(segments, Polyline{Points: pts})
		}
	}

	return segments
}

func coordMap(g *geo.Graph) map[int64]geo.LatLng {
	m := make(map[int64]geo.LatLng, len(g.Nodes))
	for _, n := range g.Nodes {
		m[n.ID] = n.LatLng
	}

	return m
}
