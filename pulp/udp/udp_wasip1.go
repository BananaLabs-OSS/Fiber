//go:build wasip1

package udp

//go:wasmimport pulp udp_listen
func hostListen(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp udp_send
func hostSend(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp udp_close
func hostClose(reqPtr, reqLen uint32) uint32
