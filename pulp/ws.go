package pulp

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// WS groups host-import wrappers for the transport.ws.inbound
// capability. The cell must declare "transport.ws.inbound" in its
// manifest for these to bind.
var WS = wsAPI{}

type wsAPI struct{}

//go:wasmimport pulp ws_register
func hostWSRegister(ptr, ln uint32) uint32

//go:wasmimport pulp ws_send
func hostWSSend(ptr, ln uint32) uint32

//go:wasmimport pulp ws_close
func hostWSClose(ptr, ln uint32) uint32

// Register declares an inbound WebSocket route. The host begins
// routing upgrade requests to this cell after Register returns.
func (wsAPI) Register(path string) error {
	p := []byte(path)
	code := hostWSRegister(uint32(uintptr(unsafe.Pointer(&p[0]))), uint32(len(p)))
	runtime.KeepAlive(p)
	if code != 0 {
		return fmt.Errorf("ws_register host code %d", code)
	}
	return nil
}

// Send delivers a frame to the identified connection.
func (wsAPI) Send(req WSSendRequest) error {
	data, err := msgpack.Marshal(req)
	if err != nil {
		return fmt.Errorf("encode ws_send: %w", err)
	}
	code := hostWSSend(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	if code != 0 {
		return fmt.Errorf("ws_send host code %d", code)
	}
	return nil
}

// Close closes the identified connection with a status code and
// reason. Code 0 is translated by the host into 1000 (normal closure).
func (wsAPI) Close(req WSCloseRequest) error {
	data, err := msgpack.Marshal(req)
	if err != nil {
		return fmt.Errorf("encode ws_close: %w", err)
	}
	code := hostWSClose(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	if code != 0 {
		return fmt.Errorf("ws_close host code %d", code)
	}
	return nil
}

// WSSendRequest and WSCloseRequest types live in abi.go — reused here.
