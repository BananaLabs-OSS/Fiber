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

type SSEAudience string

const (
	SSEAudiencePublic  SSEAudience = "public"
	SSEAudienceAccount SSEAudience = "account"
	SSEAudiencePool    SSEAudience = "pool"
)

type SecureSSERoute struct {
	Pattern     string
	Audience    SSEAudience
	MaxClients  uint32
	QueueDepth  uint32
	ReplayLimit uint32
}

// RegisterSecure registers the subscriber-aware v2 transport. Authorization
// is performed by the fixed host ABI provider before the stream is accepted.
func (sseAPI) RegisterSecure(route SecureSSERoute) error {
	req := struct {
		Version     string      `msgpack:"version"`
		Pattern     string      `msgpack:"pattern"`
		Audience    SSEAudience `msgpack:"audience"`
		MaxClients  uint32      `msgpack:"max_clients"`
		QueueDepth  uint32      `msgpack:"queue_depth"`
		ReplayLimit uint32      `msgpack:"replay_limit"`
	}{"pulp.sse.v2", route.Pattern, route.Audience, route.MaxClients, route.QueueDepth, route.ReplayLimit}
	body, err := msgpack.Marshal(req)
	if err != nil || len(body) == 0 {
		return fmt.Errorf("encode secure sse route: %w", err)
	}
	code := hostSSERegisterV2(uint32(uintptr(unsafe.Pointer(&body[0]))), uint32(len(body)))
	runtime.KeepAlive(body)
	if code != 0 {
		return fmt.Errorf("sse_register_v2 host code %d", code)
	}
	return nil
}

type SecureSSEEvent struct {
	Audience SSEAudience
	Binding  string
	Topic    string
	ID       string
	Event    string
	Data     string
}

func (sseAPI) EmitSecure(event SecureSSEEvent) error {
	req := struct {
		Version  string `msgpack:"version"`
		Audience string `msgpack:"audience"`
		Binding  string `msgpack:"binding"`
		Topic    string `msgpack:"topic,omitempty"`
		ID       string `msgpack:"id,omitempty"`
		Event    string `msgpack:"event,omitempty"`
		Data     string `msgpack:"data"`
	}{"pulp.sse.v2", string(event.Audience), event.Binding, event.Topic, event.ID, event.Event, event.Data}
	body, err := msgpack.Marshal(req)
	if err != nil || len(body) == 0 {
		return fmt.Errorf("encode secure sse event: %w", err)
	}
	code := hostSSEEmitV2(uint32(uintptr(unsafe.Pointer(&body[0]))), uint32(len(body)))
	runtime.KeepAlive(body)
	if code != 0 {
		return fmt.Errorf("sse_emit_v2 host code %d", code)
	}
	return nil
}

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
