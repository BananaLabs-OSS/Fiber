//go:build wasip1

package pulp

//go:wasmimport pulp pty_open
func hostPTYOpen(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp pty_write
func hostPTYWrite(id, dataPtr, dataLen uint32) uint32

//go:wasmimport pulp pty_resize
func hostPTYResize(id, cols, rows uint32) uint32

//go:wasmimport pulp pty_close
func hostPTYClose(id uint32) uint32

//go:wasmimport pulp pty_alive
func hostPTYAlive(id uint32) uint32
