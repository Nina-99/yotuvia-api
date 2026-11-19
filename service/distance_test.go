package service

// Distance and proximity maths: the feed's only honest use of location is
// "could this person actually go there?".

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHaversine_SameCity(t *testing.T) {
	// Santa Cruz de la Sierra: 0.0233 degrees of latitude. One degree of latitude
	// is ~111.2 km, so this is ~2.59 km (not the 3.6 I guessed — the assertion
	// caught the guess, which is what assertions are for).
	km := HaversineKM(-17.7833, -63.1821, -17.7600, -63.1821)
	assert.InDelta(t, 2.59, km, 0.10, "0.0233 degrees of latitude is ~2.59 km")
}

func TestHaversine_KnownCityPair(t *testing.T) {
	// Santa Cruz -> La Paz: ~547 km in a straight line (the road is ~550, so the
	// spherical distance is plausible, which is the sanity check that matters).
	km := HaversineKM(-17.7833, -63.1821, -16.5000, -68.1500)
	assert.InDelta(t, 547, km, 15)
}

func TestHaversine_ZeroDistance(t *testing.T) {
	assert.Equal(t, 0.0, HaversineKM(-17.78, -63.18, -17.78, -63.18))
}

func TestGeoPoint_ValidRejectsTheDefault(t *testing.T) {
	cases := []struct {
		name  string
		point *GeoPoint
		want  bool
	}{
		{"nil", nil, false},
		{"zero is 'never set', not the Gulf of Guinea", &GeoPoint{0, 0}, false},
		{"a real city", &GeoPoint{-17.78, -63.18}, true},
		{"latitude out of range", &GeoPoint{91, 0}, false},
		{"longitude out of range", &GeoPoint{0, 181}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.point.Valid())
		})
	}
}

// TestProximityScore_DecaysWithDistance — the bonus halves every
// ProximityHalfLifeKm and never goes negative.
func TestProximityScore_DecaysWithDistance(t *testing.T) {
	at := func(km float64) float64 { return ProximityScore(&km) }

	assert.Equal(t, ProximityScale, at(0))
	assert.InDelta(t, ProximityScale/2, at(ProximityHalfLifeKm), 1e-9)
	assert.InDelta(t, ProximityScale/4, at(2*ProximityHalfLifeKm), 1e-9)
	assert.Greater(t, at(10), at(40), "closer is always worth more")
	assert.Less(t, at(200), 0.05, "another region contributes essentially nothing")
	assert.GreaterOrEqual(t, at(10_000), 0.0, "the bonus is never negative")
}

func TestProximityScore_UnknownDistanceIsNeutral(t *testing.T) {
	assert.Zero(t, ProximityScore(nil), "no viewer position must not penalise anyone")

	negative := -1.0
	assert.Zero(t, ProximityScore(&negative))
}

// TestProximityBeatsPopularityElsewhere is the ordering rule the feed needs:
// a local post with no traction outranks a much more popular post from another
// region, because the popular one can never convert.
func TestProximityBeatsPopularityElsewhere(t *testing.T) {
	local := ProximityScore(ptrFloat(2)) + QualityScore(0, 0) + PopularityScore(0)
	remote := ProximityScore(ptrFloat(500)) + QualityScore(1000, 0) + PopularityScore(1000)

	assert.Greater(t, local, remote,
		"2 km away with zero views must beat 500 km away with 1000 views")
}

func ptrFloat(v float64) *float64 { return &v }

// Guard: the decay must be monotone (a regression here silently scrambles the
// whole feed's local ordering).
func TestProximityScore_Monotone(t *testing.T) {
	prev := math.Inf(1)
	for km := 0.0; km <= 200; km += 5 {
		score := ProximityScore(ptrFloat(km))
		assert.LessOrEqual(t, score, prev, "score must not grow with distance at %v km", km)
		prev = score
	}
}
