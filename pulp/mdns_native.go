//go:build !wasip1

package pulp

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostMDNSBrowse(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostMDNSAnnounce(reqPtr, reqLen uint32) uint32 { return 99 }
