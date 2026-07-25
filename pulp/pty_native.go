//go:build !wasip1

package pulp

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostPTYOpen(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostPTYWrite(id, dataPtr, dataLen uint32) uint32 { return 99 }

func hostPTYResize(id, cols, rows uint32) uint32 { return 99 }

func hostPTYClose(id uint32) uint32 { return 99 }

func hostPTYAlive(id uint32) uint32 { return 99 }
