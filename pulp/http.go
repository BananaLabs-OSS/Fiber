package pulp

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// HTTP groups host-import wrappers for the transport.http.{inbound,
// outbound} capabilities. Only the functions a cell's manifest
// declares are safe to call — the others will fail with a host trap.
var HTTP = httpAPI{}

type httpAPI struct{}

// Listen declares the bind address this cell's HTTP routes register
// against. Call once at cell init, before Register. Multiple cells
// calling Listen with the same addr share a listener (routes keyed by
// cell). Different addrs produce independent listeners.
//
// Cells that never call Listen inherit the host's default server
// bound from the HTTP_PORT env var — keeping single-cell deployments
// source-compatible with pre-multi-server behavior.
func (httpAPI) Listen(addr string) error {
	reg := struct {
		Addr string `msgpack:"addr"`
	}{Addr: addr}
	data, err := msgpack.Marshal(reg)
	if err != nil {
		return fmt.Errorf("encode listen: %w", err)
	}
	code := hostHTTPListen(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	if code != 0 {
		return fmt.Errorf("http_listen host code %d", code)
	}
	return nil
}

// Register declares an inbound route. The host begins routing matching
// requests to this cell after Register returns. Patterns support
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

// HTTPRespondStream answers an inbound request by SPLICING an open fetch
// stream (from FetchStream) straight to the client — no buffering in the cell.
// The host takes ownership of StreamID: after RespondStream the cell must NOT
// Read or Close that StreamResponse. Use for SSE / chunked / long-lived
// upstream responses, which the one-shot Respond + 30s inbound timeout cannot
// carry. Status/Headers/Cookies are written to the client before the body.
type HTTPRespondStream struct {
	ID       uint64            `msgpack:"id"`
	StreamID uint64            `msgpack:"stream_id"`
	Status   uint32            `msgpack:"status"`
	Headers  map[string]string `msgpack:"headers"`
	Cookies  []string          `msgpack:"cookies,omitempty"`
}

// RespondStream hands the host a streaming-response directive: it splices the
// already-open fetch StreamID to the waiting client and copies the body with
// per-chunk flush, exempt from the inbound timeout. Returns before the body
// finishes (the host streams in its own goroutine). The cell must not touch
// the spliced stream afterward.
func (httpAPI) RespondStream(meta HTTPRespondStream) error {
	data, err := msgpack.Marshal(meta)
	if err != nil {
		return fmt.Errorf("encode respond-stream: %w", err)
	}
	code := hostHTTPRespondStream(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	if code == 99 {
		return ErrCapabilityUnavailable
	}
	if code != 0 {
		return fmt.Errorf("http_respond_stream host code %d", code)
	}
	return nil
}

// Fetch performs an outbound HTTP request synchronously. The host
// enforces a default 30s timeout unless req.Timeout is set (non-zero),
// in which case the host applies that duration instead. Network errors
// and non-200 responses are returned through HTTPResponse.Status, not
// as a Go error. The timeout field is serialized as int64 nanoseconds
// on the wire — time.Duration encodes naturally to the host's matching
// int64 type.
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
	if code == 99 {
		return HTTPResponse{}, ErrCapabilityUnavailable
	}
	if code != 0 {
		return HTTPResponse{}, fmt.Errorf("http_fetch host code %d", code)
	}
	if respLen == 0 {
		return HTTPResponse{}, fmt.Errorf("http_fetch returned empty body")
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	buf := make([]byte, respLen)
	copy(buf, respBytes)
	releaseHostAlloc(respPtr, respLen)
	var resp HTTPResponse
	if err := msgpack.Unmarshal(buf, &resp); err != nil {
		return HTTPResponse{}, fmt.Errorf("decode fetch resp: %w", err)
	}
	return resp, nil
}

// FetchStream performs an outbound HTTP request and returns a streaming
// body reader. Use this for responses where the body may exceed the
// cell's working memory — large file downloads, world-data transfers,
// long-running streaming endpoints. The host opens the request, reads
// status + headers, and hands back a handle; each Read on the returned
// reader pulls one chunk (default chunk size 256 KiB, override via
// FetchStreamOptions.ChunkSize).
//
// The caller MUST Close the returned body to release the host-side
// stream — io.Copy + defer body.Close() is the canonical shape.
// Forgetting to Close leaks a TCP connection on the host side.
//
// Status code and headers are populated in the returned StreamResponse
// before any body bytes flow. Non-2xx status codes are NOT errors here;
// the caller inspects Status and decides.
func (httpAPI) FetchStream(req HTTPFetchRequest) (*StreamResponse, error) {
	return HTTP.FetchStreamWith(req, FetchStreamOptions{})
}

// FetchStreamOptions tunes the streaming reader. ChunkSize is the
// per-read host-buffer size in bytes — larger = fewer host crossings,
// smaller = lower peak memory. Default 256 KiB. Hard ceiling enforced
// by the host is 4 MiB.
type FetchStreamOptions struct {
	ChunkSize uint32
}

// FetchStreamWith is FetchStream with explicit options.
func (httpAPI) FetchStreamWith(req HTTPFetchRequest, opts FetchStreamOptions) (*StreamResponse, error) {
	reqBytes, err := msgpack.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode fetch: %w", err)
	}

	var hdrPtr, hdrLen uint32
	code := hostHTTPFetchBegin(
		uint32(uintptr(unsafe.Pointer(&reqBytes[0]))),
		uint32(len(reqBytes)),
		uint32(uintptr(unsafe.Pointer(&hdrPtr))),
		uint32(uintptr(unsafe.Pointer(&hdrLen))),
	)
	runtime.KeepAlive(reqBytes)
	if code == 99 {
		return nil, ErrCapabilityUnavailable
	}
	if code != 0 {
		return nil, fmt.Errorf("http_fetch_begin host code %d", code)
	}
	if hdrLen == 0 {
		return nil, errors.New("http_fetch_begin returned empty header")
	}
	hdrBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(hdrPtr))), hdrLen)
	hdrBuf := make([]byte, hdrLen)
	copy(hdrBuf, hdrBytes)
	releaseHostAlloc(hdrPtr, hdrLen)
	var hdr struct {
		ID      uint64            `msgpack:"id"`
		Status  uint32            `msgpack:"status"`
		Headers map[string]string `msgpack:"headers"`
	}
	if err := msgpack.Unmarshal(hdrBuf, &hdr); err != nil {
		return nil, fmt.Errorf("decode stream header: %w", err)
	}
	chunk := opts.ChunkSize
	if chunk == 0 {
		chunk = 256 * 1024
	}
	return &StreamResponse{
		ID:        hdr.ID,
		Status:    hdr.Status,
		Headers:   hdr.Headers,
		chunkSize: chunk,
	}, nil
}

// StreamResponse is the result of FetchStream. Status and Headers are
// populated before any body bytes flow. The embedded reader is the
// streaming body — read with io.Copy, json.Decoder, etc., then Close.
type StreamResponse struct {
	ID      uint64
	Status  uint32
	Headers map[string]string

	chunkSize uint32
	// buf holds bytes pulled from the last host read that the caller
	// hasn't consumed yet.
	buf     []byte
	eof     bool
	closed  bool
	hostErr error
}

// Read implements io.Reader. Pulls chunks from the host on demand and
// hands them to p. Returns io.EOF once the host signals the body is
// drained. Subsequent reads keep returning io.EOF; the caller still
// must Close to release the host-side stream.
func (s *StreamResponse) Read(p []byte) (int, error) {
	if s.closed {
		return 0, errors.New("read on closed StreamResponse")
	}
	if len(p) == 0 {
		return 0, nil
	}
	for len(s.buf) == 0 {
		if s.hostErr != nil {
			return 0, s.hostErr
		}
		if s.eof {
			return 0, io.EOF
		}
		if err := s.pullChunk(); err != nil {
			return 0, err
		}
	}
	n := copy(p, s.buf)
	s.buf = s.buf[n:]
	return n, nil
}

// pullChunk asks the host for the next slab of body bytes. Stores them
// in s.buf so subsequent Reads can drain incrementally.
func (s *StreamResponse) pullChunk() error {
	var chunkPtr, chunkLen uint32
	idLo := uint32(s.ID & 0xFFFFFFFF)
	idHi := uint32(s.ID >> 32)
	code := hostHTTPFetchRead(
		idLo, idHi,
		s.chunkSize,
		uint32(uintptr(unsafe.Pointer(&chunkPtr))),
		uint32(uintptr(unsafe.Pointer(&chunkLen))),
	)
	if code != 0 {
		return fmt.Errorf("http_fetch_read host code %d", code)
	}
	if chunkLen == 0 {
		// Shouldn't happen — host always writes at least an encoded
		// HTTPFetchChunk with eof flag. Treat as terminal.
		s.eof = true
		return nil
	}
	chunkBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(chunkPtr))), chunkLen)
	chunkBuf := make([]byte, chunkLen)
	copy(chunkBuf, chunkBytes)
	releaseHostAlloc(chunkPtr, chunkLen)
	var c struct {
		Bytes []byte `msgpack:"bytes"`
		EOF   bool   `msgpack:"eof"`
		Err   string `msgpack:"err,omitempty"`
	}
	if err := msgpack.Unmarshal(chunkBuf, &c); err != nil {
		return fmt.Errorf("decode chunk: %w", err)
	}
	if len(c.Bytes) > 0 {
		// Copy out — the cell's pulp_alloc memory is reused by the
		// host across calls. Holding the slice past the next host
		// crossing would be a use-after-free.
		s.buf = append(s.buf[:0:0], c.Bytes...)
	}
	if c.EOF {
		s.eof = true
	}
	if c.Err != "" {
		s.hostErr = errors.New(c.Err)
	}
	return nil
}

// Close releases the host-side stream. Always safe to call; idempotent.
func (s *StreamResponse) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	idLo := uint32(s.ID & 0xFFFFFFFF)
	idHi := uint32(s.ID >> 32)
	code := hostHTTPFetchClose(idLo, idHi)
	if code != 0 && code != 99 {
		return fmt.Errorf("http_fetch_close host code %d", code)
	}
	return nil
}
