package pulp

// Cell-side client for the transport.ws.outbound capability (Pulp-ext-wsout).
// The cell is sandboxed and cannot open sockets, so it asks the host to DIAL an
// outbound WebSocket on its behalf. The cell dials a URL, sends frames, and
// receives the remote's inbound frames as "wsout.frame" step events (decode with
// DecodeWSOutFrame). This bridges a browser's inbound WS to a remote machine's
// WS — the remote-terminal relay. The cell must declare "transport.ws.outbound"
// in its manifest.

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// WSOut groups the host-import wrappers for the transport.ws.outbound capability.
var WSOut = wsoutAPI{}

type wsoutAPI struct{}

// WSOutFrame is the payload of a "wsout.frame" step event: a chunk of a
// connection's inbound stream, tagged with the conn id it came from.
type WSOutFrame struct {
	ConnID uint32 `msgpack:"conn_id"`
	Data   []byte `msgpack:"data"`
}

//go:wasmimport pulp wsout_dial
func hostWSOutDial(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp wsout_send
func hostWSOutSend(id, dataPtr, dataLen uint32) uint32

//go:wasmimport pulp wsout_close
func hostWSOutClose(id uint32) uint32

// Dial opens an outbound WebSocket to url (optionally with request headers) and
// returns its connection id.
func (wsoutAPI) Dial(url string, headers map[string]string) (uint32, error) {
	req, err := msgpack.Marshal(struct {
		URL     string            `msgpack:"url"`
		Headers map[string]string `msgpack:"headers"`
	}{URL: url, Headers: headers})
	if err != nil {
		return 0, err
	}
	var respPtr, respLen uint32
	code := hostWSOutDial(
		uint32(uintptr(unsafe.Pointer(&req[0]))),
		uint32(len(req)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(req)
	if code == 99 {
		return 0, ErrCapabilityUnavailable
	}
	if code != 0 {
		return 0, fmt.Errorf("wsout_dial host code %d", code)
	}
	if respLen == 0 {
		return 0, fmt.Errorf("wsout_dial: empty response")
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	buf := make([]byte, respLen)
	copy(buf, src)
	releaseHostAlloc(respPtr, respLen)
	var resp struct {
		ConnID uint32 `msgpack:"conn_id"`
	}
	if err := msgpack.Unmarshal(buf, &resp); err != nil {
		return 0, err
	}
	return resp.ConnID, nil
}

// Send writes a binary frame to an outbound WebSocket.
func (wsoutAPI) Send(connID uint32, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	code := hostWSOutSend(connID, uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	if code != 0 {
		return fmt.Errorf("wsout_send host code %d", code)
	}
	return nil
}

// Close terminates an outbound WebSocket.
func (wsoutAPI) Close(connID uint32) error {
	if code := hostWSOutClose(connID); code != 0 {
		return fmt.Errorf("wsout_close host code %d", code)
	}
	return nil
}

// DecodeWSOutFrame decodes a "wsout.frame" step event payload.
func DecodeWSOutFrame(payload []byte) (WSOutFrame, error) {
	var f WSOutFrame
	err := msgpack.Unmarshal(payload, &f)
	return f, err
}
