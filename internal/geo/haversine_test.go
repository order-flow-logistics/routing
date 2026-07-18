package geo

import (
	"math"
	"testing"
)

// distEpsilon: never assert exact float equality across the TS boundary (§3.5).
const distEpsilon = 1e-9

func TestHaversineKm(t *testing.T) {
	t.Parallel()

	// R * (pi/180) = 6371 * 0.0174532925... one degree of arc.
	const oneDegreeKm = 111.19492664455873

	tests := []struct {
		name string
		a, b LatLng
		want float64
		eps  float64
	}{
		{
			name: "identical points is zero",
			a:    LatLng{Lat: 50.4501, Lng: 30.5234},
			b:    LatLng{Lat: 50.4501, Lng: 30.5234},
			want: 0,
			eps:  distEpsilon,
		},
		{
			name: "one degree of longitude on the equator",
			a:    LatLng{Lat: 0, Lng: 0},
			b:    LatLng{Lat: 0, Lng: 1},
			want: oneDegreeKm,
			eps:  1e-6,
		},
		{
			name: "one degree of latitude",
			a:    LatLng{Lat: 0, Lng: 0},
			b:    LatLng{Lat: 1, Lng: 0},
			want: oneDegreeKm,
			eps:  1e-6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := HaversineKm(tt.a, tt.b)
			if math.Abs(got-tt.want) > tt.eps {
				t.Errorf("HaversineKm(%+v, %+v) = %.12f, want %.12f (±%g)",
					tt.a, tt.b, got, tt.want, tt.eps)
			}
		})
	}
}

// TestHaversineKm_Symmetric asserts distance is direction-independent, which the
// formula guarantees exactly (dLat/dLng are only ever squared).
func TestHaversineKm_Symmetric(t *testing.T) {
	t.Parallel()

	pairs := []struct {
		name string
		a, b LatLng
	}{
		{"kyiv-lviv", LatLng{50.4501, 30.5234}, LatLng{49.8397, 24.0297}},
		{"equator-poles", LatLng{0, 0}, LatLng{90, 0}},
		{"antimeridian", LatLng{10, 179}, LatLng{-10, -179}},
	}

	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()

			ab := HaversineKm(p.a, p.b)
			ba := HaversineKm(p.b, p.a)
			if math.Abs(ab-ba) > distEpsilon {
				t.Errorf("asymmetry: ab=%.12f ba=%.12f (Δ=%g)", ab, ba, math.Abs(ab-ba))
			}
		})
	}
}
