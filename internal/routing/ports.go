package routing

import (
	"context"
	"time"

	"github.com/order-flow-logistics/routing/internal/geo"
)

// RoadNetworkProvider fetches and builds the road graph covering the given
// points. A nil graph (with nil error) means no network is available and the
// caller must fall back to straight-line distances.
type RoadNetworkProvider interface {
	FetchGraph(ctx context.Context, points []geo.LatLng) (*geo.Graph, error)
}

// DurationProvider returns the driving duration for one leg. ok is false when
// the upstream (OSRM) had no answer, which is not treated as an error.
type DurationProvider interface {
	LegDuration(ctx context.Context, from, to geo.LatLng) (d time.Duration, ok bool, err error)
}
