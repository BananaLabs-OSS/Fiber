//go:build wasip1

package tcp

//go:wasmimport pulp tcp_listen
func hostListen(uint32, uint32, uint32, uint32) uint32

//go:wasmimport pulp tcp_connect
func hostConnect(uint32, uint32, uint32, uint32) uint32

//go:wasmimport pulp tcp_write
func hostWrite(uint32, uint32, uint32, uint32) uint32

//go:wasmimport pulp tcp_start_read
func hostStartRead(uint32, uint32) uint32

//go:wasmimport pulp tcp_bridge
func hostBridge(uint32, uint32) uint32

//go:wasmimport pulp tcp_half_close
func hostHalfClose(uint32, uint32) uint32

//go:wasmimport pulp tcp_close
func hostClose(uint32, uint32) uint32

//go:wasmimport pulp tcp_listener_close
func hostListenerClose(uint32, uint32) uint32
