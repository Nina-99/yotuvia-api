package controller

// Ad density is a product knob, but a bounded one: an out-of-range value must
// be clamped, never obeyed. A typo of `1` would make every slot an ad; a typo of
// `1000` would silently disable the product.

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBoostSlotDensity_ClampsToTheSafeRange(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want int
	}{
		{"unset uses the default", "", defaultBoostSlotDensity},
		{"garbage uses the default", "abc", defaultBoostSlotDensity},
		{"below the floor is clamped up", "1", minBoostSlotDensity},
		{"zero is clamped up", "0", minBoostSlotDensity},
		{"negative is clamped up", "-5", minBoostSlotDensity},
		{"above the ceiling is clamped down", "1000", maxBoostSlotDensity},
		{"a valid value passes through", "7", 7},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BOOST_SLOT_DENSITY", tc.env)
			assert.Equal(t, tc.want, boostSlotDensity())
			assert.GreaterOrEqual(t, boostSlotDensity(), minBoostSlotDensity)
			assert.LessOrEqual(t, boostSlotDensity(), maxBoostSlotDensity)
		})
	}
}
