//go:build !wasip1

package effect

// Native builds have no Pulp host. Tests may replace this package-local seam
// with an in-memory wire implementation.
var fleetEffectExecuteWire = func(request []byte) ([]byte, uint32) { return nil, 99 }
