//go:build !wasip1

package effect

// Native builds have no Pulp host. Tests replace this seam with an exact
// in-memory MessagePack implementation.
var storageArtifactZIPValidateWire = func([]byte) ([]byte, uint32) { return nil, 99 }
