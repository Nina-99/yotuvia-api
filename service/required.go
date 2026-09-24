package service

import (
	"fmt"
	"reflect"
	"strings"
)

// dependency pairs a human-readable name with the value being validated by
// requireDeps.
type dependency struct {
	name  string
	value any
}

func dep(name string, value any) dependency { return dependency{name: name, value: value} }

// isNilDependency reports whether v is a nil dependency. A plain `v == nil`
// comparison misses TYPED nils: a nil *T (or nil map/slice/func/chan) stored
// inside an interface value is NOT equal to nil. Reflection is required to see
// through the interface, so the nil-able kinds are checked explicitly.
func isNilDependency(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return rv.IsNil()
	default:
		return false
	}
}

// requireDeps panics when any REQUIRED dependency is nil. Constructors call it
// at the composition root (router.go is built from main before ListenAndServe),
// so a mis-wired graph fails the process at BOOTSTRAP instead of surfacing a
// nil-pointer 500 on the first request that reaches the dependency.
//
// Guard rules:
//   - List ONLY dependencies required by EVERY reachable code path of the
//     service. A dependency that is conditional (`if s.dep != nil`) or only used
//     by some public methods is a legitimate nil configuration, not a wiring
//     bug: it belongs in the constructor contract comment, never here.
//   - This runs at construction time only; it can never fire per request, so it
//     adds no request-path cost.
func requireDeps(constructor string, deps ...dependency) {
	var missing []string
	for _, d := range deps {
		if isNilDependency(d.value) {
			missing = append(missing, d.name)
		}
	}
	if len(missing) > 0 {
		panic(fmt.Sprintf(
			"service: %s: required dependency is nil: %s (wiring bug at the composition root)",
			constructor, strings.Join(missing, ", ")))
	}
}
