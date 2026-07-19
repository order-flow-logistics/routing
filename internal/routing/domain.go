// Package routing is the use-case orchestrator: per-group it fetches the road
// graph, builds the distance matrix, solves the TSP, and assembles geometry and
// per-leg durations. It depends on the pure core (geo, tsp) directly and on the
// outside world only through the ports declared in ports.go.
package routing

import (
	"time"

	"github.com/order-flow-logistics/routing/internal/geo"
	"github.com/order-flow-logistics/routing/internal/tsp"
)

// Role is passthrough waypoint metadata; the orchestrator never branches on it.
type Role int32

// Role values mirror the proto WaypointRole enum.
const (
	RoleUnspecified Role = 0
	RoleStart       Role = 1
	RolePickup      Role = 2
	RoleDelivery    Role = 3
)

// DistanceSource records whether distances came from the road graph or the
// straight-line fallback (mirrors the proto DistanceSource enum).
type DistanceSource int32

// DistanceSource values mirror the proto enum.
const (
	DistanceSourceUnspecified DistanceSource = 0
	DistanceSourceOSM         DistanceSource = 1
	DistanceSourceHaversine   DistanceSource = 2
)

// Waypoint is one input stop; passthrough fields are echoed back untouched.
type Waypoint struct {
	Role     Role
	Location geo.LatLng
	OrderID  int64
	OrgID    int64
	Address  string
}

// Group is one organization's stops; Waypoints[0] is the TSP start node.
type Group struct {
	ID        string
	Waypoints []Waypoint
}

// Request is a full compute call over independent groups.
type Request struct {
	CourierID        int64
	Groups           []Group
	IncludeGeometry  bool
	IncludeDurations bool
}

// OrderedWaypoint is an input Waypoint placed in solved order with its incoming
// leg cost. HasDuration is false when OSRM gave no duration for the leg.
type OrderedWaypoint struct {
	Waypoint
	DistanceFromPrevKm float64
	DurationFromPrev   time.Duration
	HasDuration        bool
}

// Polyline is one road-geometry segment.
type Polyline struct {
	Points []geo.LatLng
}

// ComputedRoute is the solved result for one group, waypoints in solved order.
type ComputedRoute struct {
	GroupID         string
	TotalDistanceKm float64
	Waypoints       []OrderedWaypoint
	Geometry        []Polyline
	Solver          tsp.Solver
	DistanceSource  DistanceSource
}

// Response bundles one ComputedRoute per input group, in group order.
type Response struct {
	Routes []ComputedRoute
}
