//go:build wasip1

package pulp

//go:wasmimport pulp ws_register
func hostWSRegister(ptr, ln uint32) uint32

//go:wasmimport pulp ws_send
func hostWSSend(ptr, ln uint32) uint32

//go:wasmimport pulp ws_close
func hostWSClose(ptr, ln uint32) uint32
