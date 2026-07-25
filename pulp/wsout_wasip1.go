//go:build wasip1

package pulp

//go:wasmimport pulp wsout_dial
func hostWSOutDial(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp wsout_send
func hostWSOutSend(id, dataPtr, dataLen uint32) uint32

//go:wasmimport pulp wsout_close
func hostWSOutClose(id uint32) uint32
