//go:build !wasip1

package entropy

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostEntropyRead(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }
