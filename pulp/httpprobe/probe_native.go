//go:build !wasip1

package httpprobe

// Native builds have no Pulp host. Contract tests replace this fail-closed
// seam with a deterministic host implementation.
var executeWire = func([]byte) ([]byte, uint32) { return nil, 99 }
