//go:build !wasip1

package pulp

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostHTTPListen(ptr, ln uint32) uint32 { return 99 }

func hostHTTPRegister(ptr, ln uint32) uint32 { return 99 }

func hostHTTPRespond(ptr, ln uint32) uint32 { return 99 }

func hostHTTPFetch(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostHTTPFetchBegin(reqPtr, reqLen, hdrPtrOut, hdrLenOut uint32) uint32 { return 99 }

func hostHTTPFetchRead(streamIDLo, streamIDHi, maxBytes, chunkPtrOut, chunkLenOut uint32) uint32 {
	return 99
}

func hostHTTPFetchClose(streamIDLo, streamIDHi uint32) uint32 { return 99 }

func hostHTTPRespondStream(ptr, ln uint32) uint32 { return 99 }
