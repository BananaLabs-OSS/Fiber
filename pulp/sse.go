package pulp

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// SSE groups host-import wrappers for the transport.sse capability.
// The host's SSE server holds client connections open and streams
// `text/event-stream` frames; the cell registers routes via
// SSE.Register and pushes payloads via SSE.Emit. Cells do not hold
// streaming connections themselves — WASM cannot.
//
// Typical usage:
//
//	pulp.SSE.Register("/api/queue/stream")
//	// ... later, when queue state changes:
//	pulp.SSE.Emit("/api/queue/stream", "", "queue.status", payloadJSON)
var SSE = sseAPI{}

type sseAPI struct{}

// Register declares an SSE route. The host starts accepting client
// connections on path after Register returns; incoming connections
// are held open and fed whatever the cell later Emits.
func (sseAPI) Register(path string) error {
	if path == "" {
		return fmt.Errorf("sse.Register: empty path")
	}
	b := []byte(path)
	code := hostSSERegister(uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b)))
	runtime.KeepAlive(b)
	if code != 0 {
		return fmt.Errorf("sse_register host code %d", code)
	}
	return nil
}

// SubscriberCount returns the number of clients currently connected
// to the SSE route at path. Use this when you want to avoid doing
// expensive per-connection work (e.g. extending a DB TTL) when no
// one is actually listening. Returns 0 for paths that don't match
// any registered route.
func (sseAPI) SubscriberCount(path string) (uint32, error) {
	if path == "" {
		return 0, fmt.Errorf("sse.SubscriberCount: empty path")
	}
	b := []byte(path)
	var count uint32
	code := hostSSEHasSubscribers(
		uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b)),
		uint32(uintptr(unsafe.Pointer(&count))),
	)
	runtime.KeepAlive(b)
	if code != 0 {
		return 0, fmt.Errorf("sse_has_subscribers host code %d", code)
	}
	return count, nil
}

// HasSubscribers is the bool convenience wrapper over SubscriberCount.
func (sseAPI) HasSubscribers(path string) bool {
	n, err := SSE.SubscriberCount(path)
	return err == nil && n > 0
}

// Emit pushes an event to every client currently connected to path.
// id sets the SSE "id:" field (optional; use "" to omit). event sets
// the SSE "event:" field (optional). data is the event body — typically
// JSON, but any UTF-8 string works.
func (sseAPI) Emit(path, id, event, data string) error {
	req := struct {
		Path  string `msgpack:"path"`
		ID    string `msgpack:"id,omitempty"`
		Event string `msgpack:"event,omitempty"`
		Data  string `msgpack:"data"`
	}{Path: path, ID: id, Event: event, Data: data}
	body, err := msgpack.Marshal(req)
	if err != nil {
		return fmt.Errorf("encode sse emit: %w", err)
	}
	code := hostSSEEmit(uint32(uintptr(unsafe.Pointer(&body[0]))), uint32(len(body)))
	runtime.KeepAlive(body)
	if code != 0 {
		return fmt.Errorf("sse_emit host code %d", code)
	}
	return nil
}
