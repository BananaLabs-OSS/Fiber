//go:build !wasip1

package pulp

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostWSOutDial(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostWSOutSend(id, dataPtr, dataLen uint32) uint32 { return 99 }

func hostWSOutClose(id uint32) uint32 { return 99 }
