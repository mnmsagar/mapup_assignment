// Package geo holds pure geometry validation. Coordinates are [latitude, longitude].
package geo

import (
	"fmt"
	"math"
)

var Categories = map[string]bool{
	"delivery_zone":   true,
	"restricted_zone": true,
	"toll_zone":       true,
	"customer_area":   true,
}

func ValidLatLng(lat, lng float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lng) &&
		lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

// ValidatePolygon checks a closed ring of [lat, lng] pairs. It returns every
// problem found so the client can fix them in one go.
func ValidatePolygon(coords [][]float64) []string {
	var errs []string
	if len(coords) < 4 {
		errs = append(errs, fmt.Sprintf("coordinates: need at least 4 points (3 unique + closing point), got %d", len(coords)))
	}
	for i, p := range coords {
		if len(p) != 2 {
			errs = append(errs, fmt.Sprintf("coordinates[%d]: must be a [latitude, longitude] pair", i))
			continue
		}
		if p[0] < -90 || p[0] > 90 || math.IsNaN(p[0]) {
			errs = append(errs, fmt.Sprintf("coordinates[%d]: latitude %v out of range [-90, 90]", i, p[0]))
		}
		if p[1] < -180 || p[1] > 180 || math.IsNaN(p[1]) {
			errs = append(errs, fmt.Sprintf("coordinates[%d]: longitude %v out of range [-180, 180]", i, p[1]))
		}
	}
	if len(errs) > 0 || len(coords) == 0 {
		return errs
	}

	first, last := coords[0], coords[len(coords)-1]
	if first[0] != last[0] || first[1] != last[1] {
		errs = append(errs, "coordinates: first and last points must be identical (closed polygon)")
	}

	unique := map[[2]float64]struct{}{}
	for _, p := range coords[:len(coords)-1] {
		unique[[2]float64{p[0], p[1]}] = struct{}{}
	}
	if len(unique) < 3 {
		errs = append(errs, fmt.Sprintf("coordinates: need at least 3 unique points, got %d", len(unique)))
	} else if math.Abs(ringArea(coords)) < 1e-12 {
		errs = append(errs, "coordinates: polygon has zero area (points are collinear or the edges cross)")
	}
	return errs
}

// ringArea is the planar shoelace area in degrees² (sign = orientation).
func ringArea(coords [][]float64) float64 {
	var sum float64
	for i := 0; i < len(coords)-1; i++ {
		sum += coords[i][1]*coords[i+1][0] - coords[i+1][1]*coords[i][0]
	}
	return sum / 2
}

// ContainsPoint is a ray-casting point-in-polygon test (boundary handling is
// approximate). The database uses PostGIS ST_Covers; this exists for tests and
// as a documented reference of the algorithm.
func ContainsPoint(ring [][]float64, lat, lng float64) bool {
	inside := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		yi, xi := ring[i][0], ring[i][1]
		yj, xj := ring[j][0], ring[j][1]
		if (yi > lat) != (yj > lat) && lng < (xj-xi)*(lat-yi)/(yj-yi)+xi {
			inside = !inside
		}
	}
	return inside
}
