//go:build wasip1

package pulp

//go:wasmimport pulp sse_register
func hostSSERegister(ptr, ln uint32) uint32

//go:wasmimport pulp sse_emit
func hostSSEEmit(ptr, ln uint32) uint32

//go:wasmimport pulp sse_has_subscribers
func hostSSEHasSubscribers(pathPtr, pathLen, outCountPtr uint32) uint32
