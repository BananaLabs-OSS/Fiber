//go:build !wasip1

package effect

// Native builds have no Pulp host. Tests replace these seams with a precise
// in-memory MessagePack implementation.
var storageExactObjectPresignPutWire = func([]byte) ([]byte, uint32) { return nil, 99 }
var storageExactObjectValidatePutWire = func([]byte) ([]byte, uint32) { return nil, 99 }
var storageExactObjectDeleteWire = func([]byte) ([]byte, uint32) { return nil, 99 }
