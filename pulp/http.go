package pulp

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// HTTP groups host-import wrappers for the transport.http.{inbound,
// outbound} capabilities. Only the functions a plugin's manifest
// declares are safe to call — the others will fail with a host trap.
var HTTP = httpAPI{}

type httpAPI struct{}

//go:wasmimport pulp http_register
func hostHTTPRegister(ptr, ln uint32) uint32

//go:wasmimport pulp http_respond
func hostHTTPRespond(ptr, ln uint32) uint32

//go:wasmimport pulp http_fetch
func hostHTTPFetch(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

// Register declares an inbound route. The host begins routing matching
// requests to this plugin after Register returns. Patterns support
// :param segments; matched values appear in HTTPRequest.Params.
func (httpAPI) Register(method, pattern string) error {
	reg := struct {
		Method string `msgpack:"method"`
		Path   string `msgpack:"path"`
	}{Method: method, Path: pattern}
	data, err := msgpack.Marshal(reg)
	if err != nil {
		return fmt.Errorf("encode register: %w", err)
	}
	code := hostHTTPRegister(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	if code != 0 {
		return fmt.Errorf("http_register host code %d", code)
	}
	return nil
}

// Respond delivers resp to the host, which writes it to the waiting
// HTTP client. Must be called before the step returns — the host
// returns 500 to the client otherwise.
func (httpAPI) Respond(resp HTTPResponse) error {
	data, err := msgpack.Marshal(resp)
	if err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	code := hostHTTPRespond(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	if code != 0 {
		return fmt.Errorf("http_respond host code %d", code)
	}
	return nil
}

// Fetch performs an outbound HTTP request synchronously. The host
// enforces a 30s timeout; network errors and non-200 responses are
// returned through HTTPResponse.Status, not as a Go error.
func (httpAPI) Fetch(req HTTPFetchRequest) (HTTPResponse, error) {
	reqBytes, err := msgpack.Marshal(req)
	if err != nil {
		return HTTPResponse{}, fmt.Errorf("encode fetch: %w", err)
	}

	var respPtr, respLen uint32
	code := hostHTTPFetch(
		uint32(uintptr(unsafe.Pointer(&reqBytes[0]))),
		uint32(len(reqBytes)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(reqBytes)
	if code != 0 {
		return HTTPResponse{}, fmt.Errorf("http_fetch host code %d", code)
	}
	if respLen == 0 {
		return HTTPResponse{}, fmt.Errorf("http_fetch returned empty body")
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp HTTPResponse
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return HTTPResponse{}, fmt.Errorf("decode fetch resp: %w", err)
	}
	return resp, nil
}
