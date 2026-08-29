// Package tcp is the cell-side API for Pulp's supervised network.tcp
// capability. The host owns sockets and delivers bounded lifecycle/data
// events; cells own protocol framing and routing policy.
package tcp

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	EventAccepted   = "tcp.accepted"
	EventData       = "tcp.data"
	EventHalfClosed = "tcp.half_closed"
	EventClosed     = "tcp.closed"
)

type listenRequest struct {
	Addr string `msgpack:"addr"`
}
type listenResponse struct {
	ListenerID uint64 `msgpack:"listener_id"`
	Addr       string `msgpack:"addr"`
}
type connectRequest struct {
	Addr      string `msgpack:"addr"`
	TimeoutMS int    `msgpack:"timeout_ms,omitempty"`
}
type connectResponse struct {
	ConnectionID uint64 `msgpack:"connection_id"`
	LocalAddr    string `msgpack:"local_addr"`
	RemoteAddr   string `msgpack:"remote_addr"`
}
type writeRequest struct {
	ConnectionID uint64 `msgpack:"connection_id"`
	Payload      []byte `msgpack:"payload"`
	TimeoutMS    int    `msgpack:"timeout_ms,omitempty"`
}
type writeResponse struct {
	BytesWritten int `msgpack:"bytes_written"`
}
type closeRequest struct {
	ID uint64 `msgpack:"id"`
}
type bridgeRequest struct {
	LeftID  uint64 `msgpack:"left_id"`
	RightID uint64 `msgpack:"right_id"`
}
type halfCloseRequest struct {
	ConnectionID uint64 `msgpack:"connection_id"`
	Direction    string `msgpack:"direction"`
}

type Accepted struct {
	ListenerID   uint64 `msgpack:"listener_id"`
	ConnectionID uint64 `msgpack:"connection_id"`
	LocalAddr    string `msgpack:"local_addr"`
	RemoteAddr   string `msgpack:"remote_addr"`
	AcceptedAt   int64  `msgpack:"accepted_at"`
}
type Data struct {
	ConnectionID uint64 `msgpack:"connection_id"`
	Payload      []byte `msgpack:"payload"`
	ReceivedAt   int64  `msgpack:"received_at"`
}
type Closed struct {
	ConnectionID uint64 `msgpack:"connection_id"`
	Reason       string `msgpack:"reason"`
	ClosedAt     int64  `msgpack:"closed_at"`
}
type HalfClosed struct {
	ConnectionID uint64 `msgpack:"connection_id"`
	Direction    string `msgpack:"direction"`
	At           int64  `msgpack:"at"`
}

type Listener struct {
	id       uint64
	addr     string
	onAccept func(*Connection, Accepted)
}
type Connection struct {
	id                    uint64
	localAddr, remoteAddr string
	onData                func(Data)
	onHalfClose           func(HalfClosed)
	onClose               func(Closed)
}

func (l *Listener) ID() uint64           { return l.id }
func (l *Listener) Addr() string         { return l.addr }
func (c *Connection) ID() uint64         { return c.id }
func (c *Connection) LocalAddr() string  { return c.localAddr }
func (c *Connection) RemoteAddr() string { return c.remoteAddr }

var registry = struct {
	sync.Mutex
	listeners   map[uint64]*Listener
	connections map[uint64]*Connection
}{listeners: map[uint64]*Listener{}, connections: map[uint64]*Connection{}}

func call4(op string, fn func(uint32, uint32, uint32, uint32) uint32, request, response any) error {
	b, err := msgpack.Marshal(request)
	if err != nil {
		return err
	}
	var ptr, n uint32
	code := fn(uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b)), uint32(uintptr(unsafe.Pointer(&ptr))), uint32(uintptr(unsafe.Pointer(&n))))
	runtime.KeepAlive(b)
	if err := codeError(op, code); err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%s: empty response", op)
	}
	raw := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), n)
	copyBuf := append([]byte(nil), raw...)
	pulp.ReleaseHostAlloc(ptr, n)
	return msgpack.Unmarshal(copyBuf, response)
}
func call2(op string, fn func(uint32, uint32) uint32, request any) error {
	b, err := msgpack.Marshal(request)
	if err != nil {
		return err
	}
	code := fn(uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b)))
	runtime.KeepAlive(b)
	return codeError(op, code)
}

func Listen(addr string) (*Listener, error) {
	var out listenResponse
	if err := call4("tcp_listen", hostListen, listenRequest{Addr: addr}, &out); err != nil {
		return nil, err
	}
	l := &Listener{id: out.ListenerID, addr: out.Addr}
	registry.Lock()
	registry.listeners[l.id] = l
	registry.Unlock()
	return l, nil
}
func Connect(addr string, timeoutMS int) (*Connection, error) {
	var out connectResponse
	if err := call4("tcp_connect", hostConnect, connectRequest{Addr: addr, TimeoutMS: timeoutMS}, &out); err != nil {
		return nil, err
	}
	c := &Connection{id: out.ConnectionID, localAddr: out.LocalAddr, remoteAddr: out.RemoteAddr}
	registry.Lock()
	registry.connections[c.id] = c
	registry.Unlock()
	return c, nil
}
func (c *Connection) Write(payload []byte, timeoutMS int) (int, error) {
	var out writeResponse
	if err := call4("tcp_write", hostWrite, writeRequest{ConnectionID: c.id, Payload: payload, TimeoutMS: timeoutMS}, &out); err != nil {
		return 0, err
	}
	return out.BytesWritten, nil
}
func (c *Connection) CloseWrite() error {
	return call2("tcp_half_close", hostHalfClose, halfCloseRequest{ConnectionID: c.id, Direction: "write"})
}
func (c *Connection) CloseRead() error {
	return call2("tcp_half_close", hostHalfClose, halfCloseRequest{ConnectionID: c.id, Direction: "read"})
}
func (c *Connection) Close() error {
	err := call2("tcp_close", hostClose, closeRequest{ID: c.id})
	if err == nil {
		registry.Lock()
		delete(registry.connections, c.id)
		registry.Unlock()
	}
	return err
}

// Bridge transfers both directions directly in the supervised host data
// plane. Neither connection may already have an OnData reader installed.
func Bridge(left, right *Connection) error {
	if left == nil || right == nil {
		return fmt.Errorf("tcp_bridge: both connections are required")
	}
	return call2("tcp_bridge", hostBridge, bridgeRequest{LeftID: left.id, RightID: right.id})
}
func (l *Listener) Close() error {
	err := call2("tcp_listener_close", hostListenerClose, closeRequest{ID: l.id})
	if err == nil {
		registry.Lock()
		delete(registry.listeners, l.id)
		registry.Unlock()
	}
	return err
}
func (l *Listener) OnAccept(fn func(*Connection, Accepted)) {
	registry.Lock()
	l.onAccept = fn
	registry.Unlock()
}
func (c *Connection) OnData(fn func(Data)) {
	registry.Lock()
	c.onData = fn
	registry.Unlock()
	// The host deliberately leaves reads paused until the handler exists so a
	// fast peer cannot race tcp.data ahead of tcp.accepted/tcp_connect setup.
	_ = call2("tcp_start_read", hostStartRead, closeRequest{ID: c.id})
}
func (c *Connection) OnHalfClose(fn func(HalfClosed)) {
	registry.Lock()
	c.onHalfClose = fn
	registry.Unlock()
}
func (c *Connection) OnClose(fn func(Closed)) { registry.Lock(); c.onClose = fn; registry.Unlock() }

func Dispatch(ev pulp.StepEvent) error {
	switch ev.Kind {
	case EventAccepted:
		var a Accepted
		if err := msgpack.Unmarshal(ev.Payload, &a); err != nil {
			return err
		}
		registry.Lock()
		l := registry.listeners[a.ListenerID]
		if l == nil {
			registry.Unlock()
			return nil
		}
		c := &Connection{id: a.ConnectionID, localAddr: a.LocalAddr, remoteAddr: a.RemoteAddr}
		registry.connections[c.id] = c
		handler := l.onAccept
		registry.Unlock()
		if handler != nil {
			handler(c, a)
		}
	case EventData:
		var d Data
		if err := msgpack.Unmarshal(ev.Payload, &d); err != nil {
			return err
		}
		registry.Lock()
		c := registry.connections[d.ConnectionID]
		var handler func(Data)
		if c != nil {
			handler = c.onData
		}
		registry.Unlock()
		if handler != nil {
			handler(d)
		}
	case EventClosed:
		var closed Closed
		if err := msgpack.Unmarshal(ev.Payload, &closed); err != nil {
			return err
		}
		registry.Lock()
		c := registry.connections[closed.ConnectionID]
		delete(registry.connections, closed.ConnectionID)
		var handler func(Closed)
		if c != nil {
			handler = c.onClose
		}
		registry.Unlock()
		if handler != nil {
			handler(closed)
		}
	case EventHalfClosed:
		var half HalfClosed
		if err := msgpack.Unmarshal(ev.Payload, &half); err != nil {
			return err
		}
		registry.Lock()
		c := registry.connections[half.ConnectionID]
		var handler func(HalfClosed)
		if c != nil {
			handler = c.onHalfClose
		}
		registry.Unlock()
		if handler != nil {
			handler(half)
		}
	}
	return nil
}

func codeError(op string, code uint32) error {
	if code == 0 {
		return nil
	}
	if code == 99 {
		return fmt.Errorf("%s: %w", op, pulp.ErrCapabilityUnavailable)
	}
	return fmt.Errorf("%s: host code %d", op, code)
}
