//go:build wasip1

package pulp

//go:wasmimport pulp mdns_browse
func hostMDNSBrowse(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp mdns_announce
func hostMDNSAnnounce(reqPtr, reqLen uint32) uint32
