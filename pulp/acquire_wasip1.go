//go:build wasip1

package pulp

//go:wasmimport pulp tool_acquire
func hostToolAcquire(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32
