//go:build !wasip1

package udp

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostListen(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostSend(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostClose(reqPtr, reqLen uint32) uint32 { return 99 }
