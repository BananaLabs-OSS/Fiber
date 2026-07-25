//go:build !wasip1

package pulp

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostToolAcquire(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }
