//go:build !wasip1

package docker

// Native builds do not have a Pulp host. The import fails closed while the
// package-local wire seam permits deterministic contract tests.
func hostGetOwned(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func callHostGetOwnedWire(request []byte) ([]byte, uint32) {
	return nil, hostGetOwned(0, uint32(len(request)), 0, 0)
}

var dockerGetOwnedWire = callHostGetOwnedWire
