// Package pulp provides cell-authoring helpers for the Pulp application
// runtime. Import this package and your Go code becomes a valid Pulp
// cell — the required WASM exports (pulp_alloc, pulp_init, pulp_step,
// pulp_shutdown) are provided here; you register behavior with OnInit
// and OnStep. Host imports (HTTP, fs, sqlite) are wrapped as idiomatic
// Go functions.
//
// Build:
//
//	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o cell.wasm .
package pulp

import (
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

// Event kinds delivered to OnStep via StepEvent.Kind. Matches the host's
// wire format exactly — do not renumber or rename without updating Pulp.
const (
	EventHTTPRequest = "http.request"
	EventWSOpen      = "ws.open"
	EventWSFrame     = "ws.frame"
	EventWSClose     = "ws.close"
)

// StepEvent is the decoded event delivered on every step that carries a
// payload. Kind selects how to interpret Payload; Tick steps (idle with
// no event) do not fire StepHandler at all.
type StepEvent struct {
	Kind    string             `msgpack:"kind"`
	Payload msgpack.RawMessage `msgpack:"payload"`
	// WallTime is the unix-nanosecond timestamp the host set when it
	// invoked pulp_step. Cells that run periodic work use this
	// instead of a host clock import.
	WallTime uint64
	// CallNumber is the monotonically-increasing step counter.
	CallNumber uint64
}

// HTTPRequest is delivered as StepEvent.Payload when Kind is
// EventHTTPRequest. The cell is expected to call HTTP.Respond with
// this ID before returning from Step.
type HTTPRequest struct {
	ID      uint64            `msgpack:"id"`
	Method  string            `msgpack:"method"`
	Path    string            `msgpack:"path"`
	Params  map[string]string `msgpack:"params"`
	Query   map[string]string `msgpack:"query"`
	Headers map[string]string `msgpack:"headers"`
	Body    []byte            `msgpack:"body"`
	// RemoteAddr is the peer address the host observed on the incoming
	// connection ("host:port"). Populated by the host; empty when
	// unavailable. Use Context.ClientIP() — it prefers proxy headers
	// and falls back to this.
	RemoteAddr string `msgpack:"remote_addr,omitempty"`
}

// HTTPResponse is what you pass to HTTP.Respond. Status defaults to 200
// when zero. ID must match the HTTPRequest you are answering. Cookies
// carries fully-formatted Set-Cookie header values — one per entry —
// so handlers can emit multiple cookies in a single response (headers
// is a single-valued map, which would overwrite duplicates).
type HTTPResponse struct {
	ID      uint64            `msgpack:"id"`
	Status  uint32            `msgpack:"status"`
	Headers map[string]string `msgpack:"headers"`
	Cookies []string          `msgpack:"cookies,omitempty"`
	Body    []byte            `msgpack:"body"`
}

// HTTPFetchRequest is what you pass to HTTP.Fetch to perform an outbound
// HTTP call. The host executes synchronously and returns an HTTPResponse.
//
// Timeout overrides the host's default 30s timeout for this one call. Zero
// means "use host default." Useful for services that need tight latency
// budgets (e.g. 5s for matchmaking lookups) or long-running transfers
// (e.g. 10min for world archive pulls). The host applies the timeout via
// context.WithTimeout on http.Client.Do — cancellation crosses the WASM
// boundary cleanly, unlike a post-hoc wall-clock check.
type HTTPFetchRequest struct {
	Method  string            `msgpack:"method"`
	URL     string            `msgpack:"url"`
	Headers map[string]string `msgpack:"headers"`
	Body    []byte            `msgpack:"body"`
	Timeout time.Duration     `msgpack:"timeout,omitempty"`
}

// WSOpen / WSFrame / WSClose are decoded payloads for the matching
// StepEvent kinds. Cells receive them when transport.ws.inbound is
// declared in the manifest.
type WSOpen struct {
	ConnID  uint64            `msgpack:"conn_id"`
	Path    string            `msgpack:"path"`
	Query   map[string]string `msgpack:"query"`
	Headers map[string]string `msgpack:"headers"`
}

type WSFrame struct {
	ConnID  uint64 `msgpack:"conn_id"`
	OpCode  uint8  `msgpack:"opcode"`
	Payload []byte `msgpack:"payload"`
}

type WSClose struct {
	ConnID uint64 `msgpack:"conn_id"`
	Code   uint16 `msgpack:"code"`
	Reason string `msgpack:"reason"`
}

// WebSocket frame opcodes.
const (
	WSOpCodeText   uint8 = 1
	WSOpCodeBinary uint8 = 2
)

// WSSendRequest is what the cell passes to ws_send: which connection,
// what opcode, the payload bytes.
type WSSendRequest struct {
	ConnID  uint64 `msgpack:"conn_id"`
	OpCode  uint8  `msgpack:"opcode"`
	Payload []byte `msgpack:"payload"`
}

// WSCloseRequest is what the cell passes to ws_close.
type WSCloseRequest struct {
	ConnID uint64 `msgpack:"conn_id"`
	Code   uint16 `msgpack:"code"`
	Reason string `msgpack:"reason"`
}
