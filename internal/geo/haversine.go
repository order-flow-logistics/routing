// Package geo provides the pure geometric and graph primitives for routing:
// haversine distance, the OSM road graph, Dijkstra, and the distance matrix.
// It has zero non-stdlib dependencies and does no I/O, so it stays deterministic
package geo

import "math"

// EarthRadiusKm matches the TS reference (R = 6371) to keep distances in parity.
const EarthRadiusKm = 6371.0

// LatLng is a WGS84 coordinate in decimal degrees.
type LatLng struct {
	Lat float64
	Lng float64
}

func toRad(deg float64) float64 { return deg * (math.Pi / 180) }

// HaversineKm returns the great-circle distance in km between a and b.
// It mirrors the TS haversineKm formula op-for-op so results stay within epsilon.
func HaversineKm(a, b LatLng) float64 {
	dLat := toRad(b.Lat - a.Lat)
	dLng := toRad(b.Lng - a.Lng)
	sinDLat := math.Sin(dLat / 2)
	sinDLng := math.Sin(dLng / 2)
	h := sinDLat*sinDLat +
		math.Cos(toRad(a.Lat))*math.Cos(toRad(b.Lat))*sinDLng*sinDLng

	return 2 * EarthRadiusKm * math.Asin(math.Sqrt(h))
}
