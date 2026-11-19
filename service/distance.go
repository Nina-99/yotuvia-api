package service

import "math"

// GeoPoint is a viewer's position, as sent by a client that has location
// permission. A nil point means "unknown": every geo-aware decision must fall
// back to its pre-geo behaviour rather than refusing to serve.
type GeoPoint struct {
	Latitude  float64
	Longitude float64
}

// Valid reports whether the point looks like real coordinates. (0,0) — the
// default for rows that never had a location — is treated as unknown: it is in
// the Gulf of Guinea, and treating it as a real position would rank every
// unset centre as "very far" or, worse, as "very close" to another unset one.
func (p *GeoPoint) Valid() bool {
	if p == nil {
		return false
	}
	if p.Latitude == 0 && p.Longitude == 0 {
		return false
	}
	return p.Latitude >= -90 && p.Latitude <= 90 && p.Longitude >= -180 && p.Longitude <= 180
}

// EarthRadiusKm is the mean radius used by the haversine helper.
const EarthRadiusKm = 6371.0

// HaversineKM returns the great-circle distance between two points in kilometres.
//
// This is the same formula FindNearby runs in SQL, duplicated in Go on purpose:
// the feed ranks up to a thousand candidates that are already loaded, and a
// per-row SQL call would be a thousand round trips. For ranking and for a
// radius filter (tens of kilometres) the spherical approximation is exact
// enough; it is not used for billing or any exact distance claim.
func HaversineKM(aLat, aLng, bLat, bLng float64) float64 {
	toRad := math.Pi / 180
	dLat := (bLat - aLat) * toRad
	dLng := (bLng - aLng) * toRad
	lat1 := aLat * toRad
	lat2 := bLat * toRad

	sinLat := math.Sin(dLat / 2)
	sinLng := math.Sin(dLng / 2)
	h := sinLat*sinLat + math.Cos(lat1)*math.Cos(lat2)*sinLng*sinLng
	if h > 1 {
		h = 1
	}
	return 2 * EarthRadiusKm * math.Asin(math.Sqrt(h))
}
