//go:build !wasip1

package effect

// Native builds have no Pulp host. Tests replace this seam with a canonical
// MessagePack implementation.
var statusSignalPublishWire = func(request []byte) ([]byte, uint32) { return nil, 99 }
