# routing

Go microservice that computes optimized multi-stop delivery routes for
[order-flow-logistics](https://github.com/order-flow-logistics/platform).

It is a pure geo/optimization engine: given pre-grouped waypoints it fetches the
OSM road network, builds an all-pairs road-distance matrix (parallel Dijkstra),
solves the TSP (Held-Karp / greedy + 2-opt), returns the ordered route, geometry
and per-leg ETA. It knows nothing about order status, auth, or the database —
the NestJS `platform` service resolves the business problem and calls this
service over gRPC.

## Contract

The gRPC contract lives in the `platform` repo (`proto/routing/v1`) and is
published to the Buf Schema Registry as
`buf.build/order-flow-logistics/routing`. Go stubs are generated into `gen/`
via `buf generate` (see the porting plan in [`docs/porting-plan.md`](docs/porting-plan.md)).

## Layout

```
cmd/server/       entrypoint (gRPC server, graceful shutdown)
internal/geo/     haversine, OSM graph, min-heap, Dijkstra, parallel matrix
internal/tsp/     Held-Karp, greedy nearest-neighbor, 2-opt
internal/osm/     Overpass client + bounding box
internal/osrm/    per-leg driving ETA
internal/routing/ orchestrator (ComputeRoutes)
internal/server/  gRPC service impl + interceptors
gen/              generated protobuf/gRPC code (git-ignored)
```

## Development

```bash
go test ./...      # unit + parity tests
go build ./...
```

## Status

Early bootstrap. Porting the pure algorithm core (`geo`, `tsp`) from the
TypeScript implementation first, with golden parity tests, before wiring gRPC.
