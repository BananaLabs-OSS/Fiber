// Package udp is the cell-side wrapper for the network.udp capability
// provided by Pulp-ext-udp. Cell code calls these methods to open UDP
// listeners, send datagrams, and receive packets without touching host
// imports directly.
//
//	import "github.com/BananaLabs-OSS/Fiber/pulp/udp"
//
//	sock, err := udp.Listen(":9999", 256*1024)
//	sock.OnPacket(func(pkt udp.Packet) {
//	    log.Printf("got %d bytes from %s", len(pkt.Payload), pkt.SrcAddr)
//	})
//	// ... later, on an OnStep event:
//	udp.Dispatch(ev)
//	// ... and to send:
//	_, err = sock.Send("10.0.0.1:5000", payload)
//
// The cell's manifest must declare:
//
//	capabilities = ["network.udp"]
//
// and the host binary must link Pulp-ext-udp via blank import.
package udp

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

// ---------------------------------------------------------------------
// Exported event kind — mirror of Pulp-ext-udp's EventUDPPacket so
// cell code can compare against a package-local constant.
// ---------------------------------------------------------------------

// EventUDPPacket is the StepEvent kind for an inbound UDP datagram.
const EventUDPPacket = "udp.packet"

// ---------------------------------------------------------------------
// Host imports
// ---------------------------------------------------------------------

//go:wasmimport pulp udp_listen
func hostListen(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp udp_send
func hostSend(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp udp_close
func hostClose(reqPtr, reqLen uint32) uint32

// ---------------------------------------------------------------------
// Wire types — must match Pulp-ext-udp/udp.go exactly
// ---------------------------------------------------------------------

type udpListenRequest struct {
	Addr       string `msgpack:"addr"`
	BufferSize int    `msgpack:"buffer_size,omitempty"`
}

type udpListenResponse struct {
	SocketID uint64 `msgpack:"socket_id"`
}

type udpSendRequest struct {
	SocketID uint64 `msgpack:"socket_id"`
	DstAddr  string `msgpack:"dst_addr"`
	Payload  []byte `msgpack:"payload"`
}

type udpSendResponse struct {
	BytesSent int `msgpack:"bytes_sent"`
}

type udpCloseRequest struct {
	SocketID uint64 `msgpack:"socket_id"`
}

// Packet is the decoded payload for a udp.packet StepEvent.
type Packet struct {
	SocketID   uint64 `msgpack:"socket_id"`
	SrcAddr    string `msgpack:"src_addr"`
	Payload    []byte `msgpack:"payload"`
	ReceivedAt int64  `msgpack:"received_at"`
}

// ---------------------------------------------------------------------
// Socket — cell-side handle for a bound UDP socket
// ---------------------------------------------------------------------

// Socket is a cell-side handle for a bound UDP socket. Use Listen to
// obtain one, Send to transmit a datagram, OnPacket to register an
// inbound-datagram callback, and Close when finished.
type Socket struct {
	id       uint64
	addr     string
	onPacket func(Packet)
}

// ID returns the monotonic socket id assigned by the host.
func (s *Socket) ID() uint64 { return s.id }

// Addr returns the address this socket is bound to.
func (s *Socket) Addr() string { return s.addr }

// ---------------------------------------------------------------------
// Registry — so Dispatch can route inbound packets to the right Socket
// ---------------------------------------------------------------------

var (
	socketsMu sync.Mutex
	sockets   = map[uint64]*Socket{}
)

// ---------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------

// Listen binds a UDP socket at addr (e.g. ":9999" or "0.0.0.0:9999") and
// registers it in the package-local registry so Dispatch can deliver
// inbound packets via OnPacket. bufferSize is passed through to the
// host's SetReadBuffer call; pass 0 for the default.
func Listen(addr string, bufferSize int) (*Socket, error) {
	data, err := msgpack.Marshal(udpListenRequest{Addr: addr, BufferSize: bufferSize})
	if err != nil {
		return nil, fmt.Errorf("encode listen: %w", err)
	}
	var respPtr, respLen uint32
	code := hostListen(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("udp_listen", code); err != nil {
		return nil, err
	}
	if respLen == 0 {
		return nil, fmt.Errorf("udp_listen: empty response")
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp udpListenResponse
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("decode listen: %w", err)
	}
	sock := &Socket{id: resp.SocketID, addr: addr}
	socketsMu.Lock()
	sockets[resp.SocketID] = sock
	socketsMu.Unlock()
	return sock, nil
}

// Send transmits payload to dst (e.g. "10.0.0.1:5000") on this socket.
// Returns the number of bytes sent on success.
func (s *Socket) Send(dst string, payload []byte) (int, error) {
	data, err := msgpack.Marshal(udpSendRequest{
		SocketID: s.id,
		DstAddr:  dst,
		Payload:  payload,
	})
	if err != nil {
		return 0, fmt.Errorf("encode send: %w", err)
	}
	var respPtr, respLen uint32
	code := hostSend(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("udp_send", code); err != nil {
		return 0, err
	}
	if respLen == 0 {
		return 0, nil
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp udpSendResponse
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return 0, fmt.Errorf("decode send: %w", err)
	}
	return resp.BytesSent, nil
}

// Close tears down this socket on the host and removes it from the
// package-local registry.
func (s *Socket) Close() error {
	data, err := msgpack.Marshal(udpCloseRequest{SocketID: s.id})
	if err != nil {
		return fmt.Errorf("encode close: %w", err)
	}
	code := hostClose(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	if err := codeToError("udp_close", code); err != nil {
		return err
	}
	socketsMu.Lock()
	delete(sockets, s.id)
	socketsMu.Unlock()
	return nil
}

// OnPacket registers fn as the inbound-datagram handler for this socket.
// Dispatch will invoke fn for every udp.packet StepEvent whose payload
// carries this socket's id. Passing nil removes the handler.
func (s *Socket) OnPacket(fn func(Packet)) {
	socketsMu.Lock()
	s.onPacket = fn
	socketsMu.Unlock()
}

// ---------------------------------------------------------------------
// Dispatch — called from a cell's OnStep for udp.* events
// ---------------------------------------------------------------------

// Dispatch decodes a udp.packet StepEvent and delivers it to the matching
// socket's OnPacket handler. Events with other kinds are ignored, as are
// packets for sockets with no handler registered.
//
// Usage:
//
//	pulp.OnStep(func(ev pulp.StepEvent) error {
//	    if err := udp.Dispatch(ev); err != nil {
//	        return err
//	    }
//	    // ... handle other events ...
//	    return nil
//	})
func Dispatch(ev pulp.StepEvent) error {
	if ev.Kind != EventUDPPacket {
		return nil
	}
	var pkt Packet
	if err := msgpack.Unmarshal(ev.Payload, &pkt); err != nil {
		return fmt.Errorf("decode udp packet: %w", err)
	}
	socketsMu.Lock()
	sock, ok := sockets[pkt.SocketID]
	var handler func(Packet)
	if ok {
		handler = sock.onPacket
	}
	socketsMu.Unlock()
	if !ok || handler == nil {
		return nil
	}
	handler(pkt)
	return nil
}

// ---------------------------------------------------------------------
// Error mapping — mirror of docker wrapper's codeToError
// ---------------------------------------------------------------------

func codeToError(op string, code uint32) error {
	switch code {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("%s: empty request", op)
	case 2:
		return fmt.Errorf("%s: host memory read failed", op)
	case 3:
		return fmt.Errorf("%s: request decode failed", op)
	case 4:
		return fmt.Errorf("%s: host operation failed", op)
	case 5:
		return fmt.Errorf("%s: response encode failed", op)
	case 7:
		return fmt.Errorf("%s: host allocation failed", op)
	case 8:
		return fmt.Errorf("%s: host memory write failed", op)
	case 10:
		return fmt.Errorf("%s: udp manager not initialized", op)
	case 99:
		return fmt.Errorf("%s: %w", op, pulp.ErrCapabilityUnavailable)
	default:
		return fmt.Errorf("%s: host code %d", op, code)
	}
}
