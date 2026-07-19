# Routing → Go microservice: porting plan

> Status: **planning**. Project is now a pet project (post-diploma), so this is
> designed to be done *properly* — clean domain boundary, real caching, tests,
> observability, graceful degradation — not a quick extraction.
>
> Architecture decisions (locked): **pragmatic layers** (pure core + concrete
> adapters + consumer-side ports), **manual DI via constructor injection +
> functional options**, canonical module path
> **`github.com/order-flow-logistics/routing`**.

## 1. Goal & boundary

Extract the CPU-bound routing computation out of the NestJS monolith into a standalone **Go gRPC service**. The win is
genuine, not cosmetic: the current
`routing.service.ts` runs Dijkstra `n` times over an OSM graph of thousands of nodes, then Held-Karp TSP (`O(2ⁿ·n²)`) —
all on Node's single event-loop thread, blocking every other request. In Go those `n` Dijkstra runs fan out across
goroutines, and the whole thing runs natively.

### Domain split (the important decision)

| Concern                                                                                         | Owner                                 |
|-------------------------------------------------------------------------------------------------|---------------------------------------|
| Orders / couriers / organizations domain, DB reads                                              | **Nest**                              |
| Business grouping (group by org, "pending pickup?" from `OrderStatus`, courier-start injection) | **Nest**                              |
| Final `RoutePoint[]` assembly (incl. the prepended completed-pickup)                            | **Nest**                              |
| Route cache (`route:courier:<id>`) + invalidation on assignment/status change                   | **Nest**                              |
| Courier live location read (`courier:location:<id>` in Redis)                                   | **Nest** (passes it into the request) |
| OSM road-graph fetch (Overpass) + its Redis cache                                               | **Go**                                |
| Distance matrix (parallel Dijkstra)                                                             | **Go**                                |
| TSP solve (Held-Karp / greedy+2-opt)                                                            | **Go**                                |
| Road geometry (polyline per segment)                                                            | **Go**                                |
| ETA per leg (OSRM) + its Redis cache                                                            | **Go**                                |

**Go is a pure, stateless-per-request geo/optimization engine.** It knows nothing about `OrderStatus`, JWT, or the DB.
Nest resolves the *business* problem into a plain geometric "routing problem" and hands it over; Go returns optimized
orders + distances + geometry + durations; Nest maps that back onto `RoutePoint`.

This keeps `OrderStatus` semantics and all business validation (`NotFoundException` "no active deliveries",
`BadRequestException` "missing coords") on the Nest side, *before* the RPC. Go only errors on infra/compute failure.

## 2. gRPC contract

`proto/routing/v1/routing.proto`:

```proto
syntax = "proto3";

package routing.v1;

option go_package = "github.com/order-flow-logistics/routing/gen/routingv1;routingv1";

service RoutingService {
  // Compute optimized routes for a set of pre-grouped waypoint sets.
  rpc ComputeRoutes(ComputeRoutesRequest) returns (ComputeRoutesResponse);
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
}

message LatLng {
  double lat = 1;
  double lng = 2;
}

enum WaypointRole {
  WAYPOINT_ROLE_UNSPECIFIED = 0;
  WAYPOINT_ROLE_START    = 1; // courier live / last-known position (TSP start)
  WAYPOINT_ROLE_PICKUP   = 2;
  WAYPOINT_ROLE_DELIVERY = 3;
}

// Passthrough metadata is echoed back untouched so Nest can reassemble RoutePoint.
message Waypoint {
  WaypointRole role         = 1;
  LatLng       location     = 2;
  int64        order_id        = 3; // 0 when N/A
  int64        organization_id = 4; // 0 when N/A
  string       address         = 5;
}

// One group == one organization's pickup + its deliveries (+ optional start).
// waypoints[0] is the TSP start node.
message RouteGroup {
  string            group_id  = 1; // e.g. org id, for correlation
  repeated Waypoint waypoints = 2;
}

message ComputeRoutesRequest {
  int64               courier_id        = 1; // tracing/logging only
  repeated RouteGroup groups            = 2;
  bool                include_geometry  = 3; // default true
  bool                include_durations = 4; // OSRM ETA, default true
}

enum DistanceSource {
  DISTANCE_SOURCE_UNSPECIFIED = 0;
  DISTANCE_SOURCE_OSM         = 1;
  DISTANCE_SOURCE_HAVERSINE   = 2; // Overpass unavailable → straight-line fallback
}

message OrderedWaypoint {
  WaypointRole role                  = 1;
  LatLng       location              = 2;
  int64        order_id              = 3;
  int64        organization_id       = 4;
  string       address               = 5;
  double       distance_from_prev_km = 6;
  int32        duration_from_prev_sec = 7;
  bool         has_duration          = 8; // false when OSRM couldn't provide one
}

message Polyline {
  repeated LatLng points = 1;
}

message ComputedRoute {
  string                   group_id          = 1;
  double                   total_distance_km = 2;
  repeated OrderedWaypoint waypoints         = 3; // in TSP-solved order, incl. START
  repeated Polyline        geometry          = 4; // one polyline per segment; empty => null
  string                   solver            = 5; // "held-karp" | "greedy+2opt" | "greedy"
  DistanceSource           distance_source   = 6;
}

message ComputeRoutesResponse {
  repeated ComputedRoute routes = 1;
}

message HealthCheckRequest {}
message HealthCheckResponse { bool ok = 1; }
```

### Contract notes / parity with current TS behaviour

- **START handling.** Go returns waypoints in solved order *including* the START node (`distance_from_prev_km = 0`).
  When assembling `RoutePoint[]`, Nest drops the START entry — the following delivery already carries the start→delivery
  distance, exactly as `routing.service.ts:324` skips START while still crediting its outgoing segment to the next stop.
- **Geometry = per-segment polylines.** Mirrors today's
  `[number, number][][]` (`geometry`). Empty `geometry` ⇒ Nest stores `null`
  (the "OSM unavailable" case that drives the cache-staleness recompute).
- **`distance_source = HAVERSINE`** signals the Overpass-down fallback, so Nest can decide not to cache (matches the
  current "no geometry → recompute" logic).
- `has_duration=false` ⇒ leave `durationFromPrevSec` unset, as today when OSRM returns nothing.

## 3. Go service architecture

### 3.1 Architectural style: pragmatic layers

Take from clean/hexagonal only what pays for itself. The dependency rule is strict, but abstractions are introduced
**only at real seams** — never one-interface-per-package on spec.

```
┌──────────────────────────────────────────────────────────────┐
│ cmd/server/main.go     wiring: load config, build adapters,    │
│                        inject into orchestrator, serve gRPC     │
├──────────────────────────────────────────────────────────────┤
│ internal/server/       transport: gRPC impl, proto<->domain,    │
│                        interceptors (recovery, logging, timing)  │
├──────────────────────────────────────────────────────────────┤
│ internal/routing/      use-case orchestrator: per-group fan-out, │
│   service.go           calls core + adapters via PORTS           │
│   ports.go   ← consumer-side interfaces (the only seams)         │
├───────────────────────────┬──────────────────────────────────┤
│ CORE — zero project deps   │ ADAPTERS — concrete types         │
│ internal/geo/              │ internal/osm/   (Overpass HTTP)   │
│ internal/tsp/              │ internal/osrm/  (OSRM HTTP)       │
│                            │ internal/cache/ (Redis)           │
└───────────────────────────┴──────────────────────────────────┘
      dependency arrows point DOWNWARD and TOWARD the core
```

**The dependency rule.**

- `geo` and `tsp` import **nothing from this module** (no redis, no grpc, no proto, no slog). They are pure,
  deterministic, and directly unit-testable — this is what makes bit-for-bit parity with TS provable.
- `routing` imports `geo`/`tsp` **directly** (pure functions — hiding them behind interfaces would be ceremony with no
  payoff) and reaches the outside world **only through ports** it declares itself (`ports.go`).
- Adapters (`osm`, `osrm`, `cache`) are concrete structs that *happen to* satisfy those ports. They never import
  `routing`; the interface lives with the consumer, not the implementation.
- `server` translates proto⇆domain and depends on `routing`. `main.go` is the only place that knows every concrete type
  at once.

Rationale, per the Go skills: `golang-project-layout` — *"NEVER over-structure small projects"*;
`golang-design-patterns`
— *"introduce an interface only when it solves a real problem… keep the domain pure."* Three ports (network, duration,
cache) are the real seams; everything else stays concrete. A future `AssignCouriers` RPC (§6) slots in as
`internal/assign/` (another pure core) + another use-case, with no structural rework.

### 3.2 Repository layout (repo-rooted, module `github.com/order-flow-logistics/routing`)

```
routing/                              # repo root == Go module root
├── cmd/
│   └── server/
│       └── main.go                   # config load, redis, grpc.Server, graceful shutdown, wiring
├── internal/
│   ├── config/
│   │   ├── config.go                 # 12-factor env load → validated Config struct
│   │   └── config_test.go
│   ├── geo/                          # ── PURE CORE (zero project deps) ──
│   │   ├── haversine.go              # <- dijkstra.ts:haversineKm
│   │   ├── haversine_test.go
│   │   ├── graph.go                  # <- buildOsmGraph (oneway-aware), Node/Edge, findNearestNode
│   │   ├── graph_test.go
│   │   ├── minheap.go                # <- MinHeap (container/heap or hand-rolled)
│   │   ├── minheap_test.go
│   │   ├── dijkstra.go               # <- dijkstraWithPath, reconstructPath
│   │   ├── dijkstra_test.go
│   │   ├── matrix.go                 # <- computeOsmDistanceMatrix (PARALLEL) + buildHaversineMatrix
│   │   ├── matrix_test.go
│   │   └── testdata/                 # golden matrices exported from TS
│   ├── tsp/                          # ── PURE CORE (zero project deps) ──
│   │   ├── heldkarp.go               # <- heldKarpExact (bitmask DP, []float64 / []int16)
│   │   ├── heldkarp_test.go
│   │   ├── greedy.go                 # <- greedyNearestNeighbor
│   │   ├── twoopt.go                 # <- twoOptRefine (EPSILON = 1e-9)
│   │   ├── solve.go                  # <- solveTSP dispatch (HeldKarpLimit = 12)
│   │   ├── solve_test.go
│   │   ├── solve_fuzz_test.go        # invariants (permutation, start-fixed, ≤ greedy)
│   │   └── testdata/                 # golden (matrix, start) → route fixtures from TS
│   ├── routing/                      # ── USE-CASE ORCHESTRATOR ──
│   │   ├── ports.go                  # consumer-side interfaces (the seams)
│   │   ├── domain.go                 # request/result domain structs (no proto)
│   │   ├── service.go                # per-group fan-out; New(...) + functional options
│   │   └── service_test.go           # drives orchestrator with fake ports
│   ├── osm/                          # ── ADAPTER ──
│   │   ├── overpass.go               # <- OsmService.fetchRoadNetwork, 3-endpoint fallback
│   │   ├── bbox.go                   # <- computeBoundingBox, grid snapping, cache key
│   │   └── overpass_test.go          # httptest server
│   ├── osrm/                         # ── ADAPTER ──
│   │   ├── client.go                 # <- RoadDistanceService ETA (getDistanceKm), driving duration
│   │   └── client_test.go            # httptest server
│   ├── cache/                        # ── ADAPTER ──
│   │   ├── redis.go                  # go-redis wrapper (namespaced: osm:, osrm:dist:)
│   │   └── redis_test.go             # //go:build integration (miniredis or real)
│   └── server/                       # ── TRANSPORT ──
│       ├── server.go                 # RoutingService gRPC impl (ComputeRoutes, HealthCheck)
│       ├── mapping.go                # proto <-> routing domain
│       ├── mapping_test.go
│       └── interceptors.go           # recovery, structured logging, request timing
├── gen/
│   └── routingv1/                    # buf-generated pb.go + grpc.pb.go (git-ignored)
├── proto/
│   └── routing/v1/routing.proto
├── buf.yaml
├── buf.gen.yaml
├── Dockerfile                        # multi-stage → distroless
├── Makefile
├── .golangci.yml
├── .gitignore
├── go.mod                            # module github.com/order-flow-logistics/routing
└── go.sum
```

Deviations from the earlier draft, per the skills:

- **Repo-rooted, not `services/routing/`.** This repo *is* the module; `golang-project-layout` wants the module root at
  the repo root, `cmd/` for the entrypoint, `internal/` for everything private.
- **`testdata/` co-located per package**, not a top-level `test/golden/`. Go's `go test` tooling, coverage, and IDE
  navigation all resolve fixtures via each package's own `testdata/` dir (ignored by the build).
- **Test files named after their source file** (`dijkstra.go` → `dijkstra_test.go`), tests ordered to match source order
  (`golang-testing`).

### 3.3 Dependency injection: manual constructor injection + functional options

No DI container. Each package exposes a `New(...)` constructor taking its **required** dependencies as positional args
and **optional** tuning via functional options. `main.go` wires the graph by hand — the entire dependency graph is
visible in one function.

**Ports declared by the consumer** (`internal/routing/ports.go`):

```go
package routing

// RoadNetworkProvider fetches the OSM road graph for a bounding box.
type RoadNetworkProvider interface {
	FetchGraph(ctx context.Context, bbox geo.BBox) (*geo.Graph, error)
}

// DurationProvider returns the driving duration for a single leg.
// ok=false means OSRM had no answer (→ has_duration=false downstream).
type DurationProvider interface {
	LegDuration(ctx context.Context, from, to geo.LatLng) (d time.Duration, ok bool, err error)
}

// Cache is the minimal KV surface the orchestrator needs.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
}
```

**Constructor with functional options** (`internal/routing/service.go`):

```go
type Service struct {
net   RoadNetworkProvider
dur   DurationProvider
cache Cache
log   *slog.Logger

concurrency   int // bound on parallel Dijkstra sources; default runtime.NumCPU()
heldKarpLimit int // default 12
}

type Option func (*Service)

func WithConcurrency(n int) Option   { return func (s *Service) { s.concurrency = n } }
func WithHeldKarpLimit(n int) Option { return func (s *Service) { s.heldKarpLimit = n } }
func WithLogger(l *slog.Logger) Option { return func (s *Service) { s.log = l } }

func New(net RoadNetworkProvider, dur DurationProvider, cache Cache, opts ...Option) *Service {
s := &Service{
net: net, dur: dur, cache: cache,
log:           slog.Default(),
concurrency:   runtime.NumCPU(),
heldKarpLimit: 12,
}
for _, o := range opts {
o(s)
}
return s
}
```

**Compile-time port checks** live beside each adapter, so a signature drift fails at build time:

```go
// internal/osm/overpass.go
var _ routing.RoadNetworkProvider = (*Client)(nil)
```

**Wiring** (`cmd/server/main.go`, the only place that sees all concretes):

```go
cfg := config.Load() // 12-factor env; fail fast on bad config
rdb := cache.NewRedis(cfg.RedisURL)
osmClient := osm.NewClient(cfg.OverpassEndpoints, osm.WithCache(rdb), osm.WithTimeout(cfg.OverpassTimeout))
osrmClient := osrm.NewClient(cfg.OSRMBaseURL, osrm.WithCache(rdb))

svc := routing.New(osmClient, osrmClient, rdb,
routing.WithLogger(logger),
routing.WithConcurrency(cfg.MatrixConcurrency),
)
grpcSrv := server.New(svc, logger) // registers RoutingService + health
```

Constructors that can fail validation return `(*T, error)` and validate in the option-apply loop
(`golang-design-patterns`: options that validate should return errors — catch bad config at construction).

### 3.4 Concurrency: parallel matrix (the headline win)

One goroutine per source waypoint runs Dijkstra; rows are disjoint so no locking. The pure core stays **stdlib-only**
(`sync.WaitGroup` + a buffered channel as a semaphore) rather than `errgroup`: Dijkstra is pure in-memory computation
that never returns an error, so there is no error to propagate, and keeping `geo` zero-dependency is the whole point of
the pure-core boundary (§3.1). `ComputeMatrix` therefore returns just `*Matrix`, no `error`, no `context` — cancellation
and timeouts live one layer up in the orchestrator (`golang-concurrency`). Implemented in `internal/geo/matrix.go`:

```go
func ComputeMatrix(g *Graph, waypointIDs []int64, wantPaths bool, concurrency int) *Matrix {
	n := len(waypointIDs)
	m := &Matrix{Dist: make([][]float64, n)}
	if wantPaths {
		m.Paths = make([][][]int64, n)
	}
	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for i := range waypointIDs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			dist, paths := singleSourceRow(g, waypointIDs, i, wantPaths)
			m.Dist[i] = dist
			if wantPaths {
				m.Paths[i] = paths
			}
		}()
	}
	wg.Wait()
	return m
}
```

- **Group-level fan-out** — independent org groups also compute concurrently in the orchestrator; the OSM graph is
  fetched once for the shared bbox, then shared read-only across goroutines (no mutation after build ⇒ safe without
  locks). That fan-out is where `errgroup` + `context` *do* belong, because it wraps I/O (Overpass/OSRM) that can fail.
- **Determinism preserved.** Parallelism is over *disjoint output rows* and independent groups; within a source,
  Dijkstra and Held-Karp run exactly as in TS. Concurrency does not affect the produced ordering — golden parity (§5)
  still holds.

### 3.5 Core parity notes (bit-for-bit route, epsilon distance)

- **Held-Karp** — same bitmask DP; `[]float64` for `dp`, `[]int16` for `parent`, indexed `mask*n + i`.
  `n <= HeldKarpLimit`
  (12) guard stays; the `n == 1` / `n <= 1` early returns match `tsp-solvers.ts`.
- **2-opt** — port `EPSILON = 1e-9` and the exact `i`/`j` iteration + tie-break order, so the improvement sequence is
  identical to TS.
- **Dijkstra** — same `if uDist > dist[u] { continue }` stale-pop skip; `MinHeap` sift order must match so ties resolve
  identically. Prefer `container/heap` or a hand-rolled heap that reproduces TS's comparison (`<=` in bubble-up, `<` in
  sift-down) exactly.
- **Float discipline** — never compare distances with `==` across the boundary; golden tests assert routes exactly
  (integer vertex order) and distances within a small epsilon (`golang-safety`: float equality is a trap).
- **Overpass client** — same 3-endpoint fallback; `http.Client` + `context.WithTimeout` replaces `AbortSignal.timeout`.
  Drop the `"diploma project"` User-Agent string.

### 3.6 Error handling

- Sentinel errors at package boundaries (`var ErrOverpassUnavailable = errors.New(...)`, `ErrNoRoute`), wrapped with
  `%w`
  so callers use `errors.Is` / `errors.As` (`golang-error-handling`).
- The orchestrator maps `ErrOverpassUnavailable` → Haversine fallback + `DISTANCE_SOURCE_HAVERSINE` (graceful
  degradation, not an RPC error) — this is the current "OSM down → straight-line" behaviour.
- The gRPC layer maps remaining infra/compute failures to status codes: timeouts/upstream-down → `codes.Unavailable`,
  unexpected → `codes.Internal`, malformed request → `codes.InvalidArgument`. Business 404/400 never reach Go.
- **Handle each error once** — log-and-return is double handling; log at the transport edge (interceptor), return
  everywhere below.

### 3.7 Config (12-factor)

`internal/config` reads env into a validated `Config` struct and fails fast on missing/invalid values (no `init()`, no
globals). Keys: `GRPC_ADDR`, `REDIS_URL`, `OSRM_BASE_URL`, `OVERPASS_ENDPOINTS` (comma-separated),
`OVERPASS_TIMEOUT`, `MATRIX_CONCURRENCY`, `LOG_LEVEL`. Plain stdlib env parsing is enough here — no Viper/Cobra (that's
for CLIs); logs go to stdout as structured `slog` JSON.

### 3.8 Proper-project extras (now that we're not cutting corners)

- Structured logging via `log/slog`; request-scoped logger carrying `courier_id` + `group_id`, injected through context.
- gRPC `recovery` + `logging` + timing interceptors; `grpc.GracefulStop()` on SIGTERM with a drain deadline.
- Real `HealthCheck` **and** the standard gRPC health protocol (`grpc.health.v1`) for k8s/compose probes.
- `buf` for proto gen + lint + breaking-change checks in CI.
- `golangci-lint` (incl. `paralleltest`, `testifylint`, `thelper`), `go test -race` in CI.
- `Dockerfile` multi-stage → distroless final image; `Makefile` for `generate`/`test`/`lint`/`build`.

## 4. Testing strategy

Layered to match §3, per `golang-testing`:

1. **Pure-core unit tests** (`geo`, `tsp`) — table-driven with named subtests, `t.Parallel()` where independent. These
   are the bulk; they're fast (<1ms) and deterministic because the core has zero I/O.
2. **Golden parity tests** — export fixtures from the current TS (`(osmGraph, waypoints) → matrix` and
   `(matrix, start) → route`) into `internal/geo/testdata/` and `internal/tsp/testdata/`. Assert Go matches: **route
   order exactly**, **distances within epsilon** (§3.5). This is the acceptance gate for the port.
3. **Fuzz the TSP invariants** (`solve_fuzz_test.go`) — for random matrices, the solved route must be a permutation of
   all nodes, start-fixed at index 0, and no longer than the greedy baseline. Catches edge cases table tests miss.
4. **Orchestrator tests** (`routing`) — drive `Service` with **fake ports** (in-memory `RoadNetworkProvider` /
   `DurationProvider` / `Cache`), asserting fan-out, Haversine fallback, and cache behaviour without touching
   HTTP/Redis.
5. **Adapter tests** (`osm`, `osrm`) — `httptest.Server` for Overpass/OSRM incl. the 3-endpoint failover path.
6. **Goroutine-leak detection** — `goleak.VerifyTestMain` in `TestMain` for `geo` (parallel matrix) and `routing`
   (group fan-out), so a leaked goroutine fails the suite.
7. **Integration tests** behind `//go:build integration` — real Redis (or miniredis) for `cache`, kept out of the fast
   unit run and gated in CI separately.
8. **Examples** as executable docs for the public `geo`/`tsp` funcs (`ExampleSolve`), verified by `go test`.

## 5. Migration phases

1. **Scaffold** — `proto/` (with the corrected `go_package`), `buf`, gen for both Go and TS. Commit the contract first.
2. **Port pure core** — `geo/` + `tsp/` in Go with unit tests. Export golden fixtures from TS into each package's
   `testdata/` and assert parity (§4). **This is the current task and the acceptance gate.**
3. **Orchestrator + ports** — `internal/routing` with `ports.go`, `New(...)` + options, tested against fake ports.
4. **Wire I/O** — `osm` + `osrm` + `cache` adapters; stand the gRPC `server` up; `main.go` wiring; `ComputeRoutes`
   returns real routes for hand-built requests.
5. **Dual-run behind a flag** — `ROUTING_ENGINE=go|ts` in Nest. Shadow-call Go, log/compare against the TS result on
   live traffic; fix discrepancies.
6. **Cut over & delete** — flip default to `go`, remove the TS algorithm files and the flag. Add the Go service to
   `docker-compose` (+ `ROUTING_GRPC_URL` for Nest, `REDIS_URL`/`OSRM_BASE_URL`/`OVERPASS_ENDPOINTS` for Go).

## 6. NestJS-side changes

1. **Add deps:** `@grpc/grpc-js`, `@grpc/proto-loader` (contract already has `@nestjs/microservices`).
2. **New `RoutingGrpcClient`** — `ClientGrpc` pointed at `ROUTING_GRPC_URL`, loads the same `routing.proto`.
3. **`RoutingService` becomes thin.** It keeps: DB reads (`orders`, `organizations`), courier-location Redis read, route
   cache read/write, `invalidateCourierRoute`, and all business validation. It *loses* the algorithm calls. New shape of
   `getOptimizedRoute`:
    - business validation + grouping (unchanged) → build `RouteGroup[]`
    - `await this.routingGrpc.computeRoutes({ courierId, groups, ... })`
    - map `ComputedRoute[]` → `RoutePoint[]` (drop START, prepend completed pickup for `!hasPendingPickup` groups — the
      existing lines 305-316 logic stays)
    - cache + return (unchanged staleness rules)
4. **Delete from Nest** once cut over: `routing/osm.service.ts`, `routing/dijkstra.ts`, `routing/tsp-solvers.ts`.
5. **Relocate `haversineKm`.** `pricing/road-distance.service.ts:4` imports it from `routing/dijkstra`. Since routing's
   ETA moves to Go but `RoadDistanceService` still serves **pricing**, move `haversineKm` into a small shared util (e.g.
   `common/geo.ts`) and update the pricing import. (Pricing keeps its own OSRM usage; only *routing's* ETA moves to Go.)
6. **Error mapping.** Go infra failures → gRPC `UNAVAILABLE`/`INTERNAL`; Nest surfaces a 502/503. Business 404/400 never
   reach Go.

## 7. Later (optional)

Fold `courier-assignment/hungarian.ts` into the same Go service as a second RPC (`AssignCouriers`) — it's the other pure
CPU-bound optimizer. Under the pragmatic-layers design this is additive: a new `internal/assign/` pure-core package + a
new use-case + a new port set, with no restructuring — turning this into a general **optimization service**. Not
required for the routing cutover.
