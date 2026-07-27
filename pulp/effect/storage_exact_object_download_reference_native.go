//go:build !wasip1

package effect

var storageExactObjectDownloadReferenceWire = func([]byte) ([]byte, uint32) { return nil, 99 }
