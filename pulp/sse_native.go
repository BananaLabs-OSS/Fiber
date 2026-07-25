//go:build !wasip1

package pulp

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostSSERegister(ptr, ln uint32) uint32 { return 99 }

func hostSSEEmit(ptr, ln uint32) uint32 { return 99 }

func hostSSEHasSubscribers(pathPtr, pathLen, outCountPtr uint32) uint32 { return 99 }
