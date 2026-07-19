package routing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/order-flow-logistics/routing/internal/geo"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

var (
	_ RoadNetworkProvider = (*fakeNet)(nil)
	_ DurationProvider    = (*fakeDur)(nil)
)

type fakeNet struct {
	graph *geo.Graph
	err   error
}

func (f *fakeNet) FetchGraph(_ context.Context, _ []geo.LatLng) (*geo.Graph, error) {
	return f.graph, f.err
}

type fakeDur struct {
	d   time.Duration
	ok  bool
	err error
}

func (f *fakeDur) LegDuration(_ context.Context, _, _ geo.LatLng) (time.Duration, bool, error) {
	return f.d, f.ok, f.err
}

func lineGroup() Group {
	return Group{
		ID: "org-1",
		Waypoints: []Waypoint{
			{Role: RoleStart, Location: geo.LatLng{Lat: 0, Lng: 0}},
			{Role: RoleDelivery, OrderID: 100, Location: geo.LatLng{Lat: 0, Lng: 2}},
			{Role: RoleDelivery, OrderID: 200, Location: geo.LatLng{Lat: 0, Lng: 1}},
		},
	}
}

func TestComputeRoutes_HaversineFallback(t *testing.T) {
	t.Parallel()

	svc := New(&fakeNet{}, &fakeDur{}, WithConcurrency(2))

	resp, err := svc.ComputeRoutes(context.Background(), Request{Groups: []Group{lineGroup()}})
	require.NoError(t, err)
	require.Len(t, resp.Routes, 1)

	r := resp.Routes[0]
	assert.Equal(t, "org-1", r.GroupID)
	assert.Equal(t, DistanceSourceHaversine, r.DistanceSource)
	assert.Nil(t, r.Geometry)

	require.Len(t, r.Waypoints, 3)
	assert.Equal(t, RoleStart, r.Waypoints[0].Role)
	assert.Zero(t, r.Waypoints[0].DistanceFromPrevKm)

	// From start (0,0) the nearer stop (0,1)=order 200 comes before (0,2)=order 100.
	assert.Equal(t, int64(200), r.Waypoints[1].OrderID)
	assert.Equal(t, int64(100), r.Waypoints[2].OrderID)

	want := geo.HaversineKm(geo.LatLng{Lat: 0, Lng: 0}, geo.LatLng{Lat: 0, Lng: 1}) +
		geo.HaversineKm(geo.LatLng{Lat: 0, Lng: 1}, geo.LatLng{Lat: 0, Lng: 2})
	assert.InEpsilon(t, want, r.TotalDistanceKm, 1e-9)
}

func TestComputeRoutes_OSMWithGeometry(t *testing.T) {
	t.Parallel()

	nodes := []geo.OSMNode{{ID: 1, Lat: 0, Lng: 0}, {ID: 2, Lat: 0, Lng: 1}, {ID: 3, Lat: 0, Lng: 2}}
	ways := []geo.OSMWay{{ID: 1, Nodes: []int64{1, 2, 3}, Oneway: false}}

	svc := New(&fakeNet{graph: geo.BuildGraph(nodes, ways)}, &fakeDur{})

	resp, err := svc.ComputeRoutes(context.Background(), Request{
		Groups:          []Group{lineGroup()},
		IncludeGeometry: true,
	})
	require.NoError(t, err)

	r := resp.Routes[0]
	assert.Equal(t, DistanceSourceOSM, r.DistanceSource)
	require.NotEmpty(t, r.Geometry)

	for _, seg := range r.Geometry {
		assert.Greater(t, len(seg.Points), 1)
	}
}

func TestComputeRoutes_AttachesDurations(t *testing.T) {
	t.Parallel()

	svc := New(&fakeNet{}, &fakeDur{d: 90 * time.Second, ok: true})

	resp, err := svc.ComputeRoutes(context.Background(), Request{
		Groups:           []Group{lineGroup()},
		IncludeDurations: true,
	})
	require.NoError(t, err)

	r := resp.Routes[0]
	assert.False(t, r.Waypoints[0].HasDuration)

	for i := 1; i < len(r.Waypoints); i++ {
		assert.True(t, r.Waypoints[i].HasDuration)
		assert.Equal(t, 90*time.Second, r.Waypoints[i].DurationFromPrev)
	}
}

func TestComputeRoutes_DurationsOffLeavesThemUnset(t *testing.T) {
	t.Parallel()

	svc := New(&fakeNet{}, &fakeDur{d: 90 * time.Second, ok: true})

	resp, err := svc.ComputeRoutes(context.Background(), Request{Groups: []Group{lineGroup()}})
	require.NoError(t, err)

	for _, wp := range resp.Routes[0].Waypoints {
		assert.False(t, wp.HasDuration)
	}
}

func TestComputeRoutes_GraphErrorFallsBackToHaversine(t *testing.T) {
	t.Parallel()

	svc := New(&fakeNet{err: errors.New("overpass down")}, &fakeDur{})

	resp, err := svc.ComputeRoutes(context.Background(), Request{Groups: []Group{lineGroup()}})
	require.NoError(t, err)
	assert.Equal(t, DistanceSourceHaversine, resp.Routes[0].DistanceSource)
}

func TestComputeRoutes_EmptyGroup(t *testing.T) {
	t.Parallel()

	svc := New(&fakeNet{}, &fakeDur{})

	resp, err := svc.ComputeRoutes(context.Background(), Request{Groups: []Group{{ID: "empty"}}})
	require.NoError(t, err)
	require.Len(t, resp.Routes, 1)
	assert.Empty(t, resp.Routes[0].Waypoints)
	assert.Equal(t, "empty", resp.Routes[0].GroupID)
}

func TestComputeRoutes_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	svc := New(&fakeNet{}, &fakeDur{})

	_, err := svc.ComputeRoutes(ctx, Request{Groups: []Group{lineGroup()}})
	require.Error(t, err)
}
