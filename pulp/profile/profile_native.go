//go:build !wasip1

package profile

// Native builds have no host imports. The package-local seam is intentionally
// fail-closed and exists only so contract tests can supply a deterministic host.
var minecraftProfileResolveWire = func([]byte) ([]byte, uint32) { return nil, 99 }
