package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsNilDependency_DetectsTypedNil pins the reason this helper exists: a
// typed-nil interface (nil *T stored in an interface) is NOT == nil, so the
// naive comparison would let a broken wiring slip through to request time.
func TestIsNilDependency_DetectsTypedNil(t *testing.T) {
	var typedNil *strings.Reader // nil pointer
	assert.Nil(t, typedNil)
	assert.True(t, isNilDependency(nil), "untyped nil is nil")
	assert.True(t, isNilDependency(typedNil), "typed nil pointer must be detected")

	assert.False(t, isNilDependency(strings.NewReader("wired")), "non-nil value is not nil")
}

// TestRequireDeps_PanicsOnMissing names the constructor and the offending
// dependency so the bootstrap failure is actionable.
func TestRequireDeps_PanicsOnMissing(t *testing.T) {
	var typedNil *strings.Reader
	assert.PanicsWithValue(t,
		"service: NewThing: required dependency is nil: repo (wiring bug at the composition root)",
		func() {
			requireDeps("NewThing",
				dep("repo", typedNil),
				dep("validate", &strings.Reader{}),
			)
		})
}

// TestRequireDeps_AllowsWhenEveryDependencyWired is the happy path: a fully
// wired graph constructs without panicking.
func TestRequireDeps_AllowsWhenEveryDependencyWired(t *testing.T) {
	assert.NotPanics(t, func() {
		requireDeps("NewThing",
			dep("repo", &strings.Reader{}),
			dep("validate", &strings.Reader{}),
		)
	})
}
