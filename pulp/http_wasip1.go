//go:build wasip1

package pulp

//go:wasmimport pulp http_listen
func hostHTTPListen(ptr, ln uint32) uint32

//go:wasmimport pulp http_register
func hostHTTPRegister(ptr, ln uint32) uint32

//go:wasmimport pulp http_respond
func hostHTTPRespond(ptr, ln uint32) uint32

//go:wasmimport pulp http_fetch
func hostHTTPFetch(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp http_fetch_begin
func hostHTTPFetchBegin(reqPtr, reqLen, hdrPtrOut, hdrLenOut uint32) uint32

//go:wasmimport pulp http_fetch_read
func hostHTTPFetchRead(streamIDLo, streamIDHi, maxBytes, chunkPtrOut, chunkLenOut uint32) uint32

//go:wasmimport pulp http_fetch_close
func hostHTTPFetchClose(streamIDLo, streamIDHi uint32) uint32

//go:wasmimport pulp http_respond_stream
func hostHTTPRespondStream(ptr, ln uint32) uint32
