//go:build wasip1

package entropy

//go:wasmimport pulp entropy_read
func hostEntropyRead(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32
