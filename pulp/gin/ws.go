package gin

import (
	"sync"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

// WSHandlers groups the per-event callbacks for a WebSocket route.
// Any handler may be nil — events for which no handler is installed
// are silently ignored.
type WSHandlers struct {
	OnOpen  func(c *WSContext)
	OnFrame func(c *WSContext)
	OnClose func(c *WSContext)
}

// WSContext is what a WS handler receives. ConnID is always set; the
// other fields are populated for the event kind that fired:
//
//	OnOpen  — Path, Query, Headers
//	OnFrame — OpCode, Payload
//	OnClose — Code, Reason
type WSContext struct {
	ConnID  uint64
	Path    string
	Query   map[string]string
	Headers map[string]string
	OpCode  uint8
	Payload []byte
	Code    uint16
	Reason  string

	// Keys is per-connection scratch shared across the lifetime of
	// the connection (populated in OnOpen, read in OnFrame, etc.).
	// Access is safe only from the step-loop goroutine; concurrent
	// access is not expected.
	Keys map[string]any
}

// Send writes a frame on this connection.
func (c *WSContext) Send(opcode uint8, payload []byte) error {
	return pulp.WS.Send(pulp.WSSendRequest{
		ConnID:  c.ConnID,
		OpCode:  opcode,
		Payload: payload,
	})
}

// SendText is the text-frame shortcut.
func (c *WSContext) SendText(s string) error {
	return c.Send(pulp.WSOpCodeText, []byte(s))
}

// SendBinary is the binary-frame shortcut.
func (c *WSContext) SendBinary(b []byte) error {
	return c.Send(pulp.WSOpCodeBinary, b)
}

// Close closes this connection.
func (c *WSContext) Close(code uint16, reason string) error {
	return pulp.WS.Close(pulp.WSCloseRequest{
		ConnID: c.ConnID,
		Code:   code,
		Reason: reason,
	})
}

// WS registers a WebSocket route. The handler bundle's callbacks fire
// when the host delivers ws.open, ws.frame, and ws.close events for
// the matching path.
func (e *Engine) WS(path string, handlers WSHandlers) *Engine {
	e.wsRoutes = append(e.wsRoutes, wsRoute{path: path, handlers: handlers})
	return e
}

// WS on a RouterGroup prepends the group's prefix.
func (g *RouterGroup) WS(path string, handlers WSHandlers) *RouterGroup {
	g.engine.wsRoutes = append(g.engine.wsRoutes, wsRoute{
		path:     g.prefix + path,
		handlers: handlers,
	})
	return g
}

type wsRoute struct {
	path     string
	handlers WSHandlers
}

// Engine keeps its own per-connection state: conn_id → path (so frames
// and closes find the right handler bundle) plus per-connection Keys
// so handlers can persist state across a single socket's lifetime.
type wsConnState struct {
	path string
	keys map[string]any
}

var (
	wsConnsMu sync.Mutex
	wsConns   = map[uint64]*wsConnState{}
)

// dispatchWS is called from Engine.dispatch for ws.* step events.
func (e *Engine) dispatchWS(ev pulp.StepEvent) error {
	switch ev.Kind {
	case pulp.EventWSOpen:
		var open pulp.WSOpen
		if err := msgpack.Unmarshal(ev.Payload, &open); err != nil {
			return err
		}
		route := e.findWSRoute(open.Path)
		if route == nil {
			return nil
		}
		state := &wsConnState{path: open.Path, keys: map[string]any{}}
		wsConnsMu.Lock()
		wsConns[open.ConnID] = state
		wsConnsMu.Unlock()
		if route.handlers.OnOpen != nil {
			c := &WSContext{
				ConnID:  open.ConnID,
				Path:    open.Path,
				Query:   open.Query,
				Headers: open.Headers,
				Keys:    state.keys,
			}
			route.handlers.OnOpen(c)
		}
	case pulp.EventWSFrame:
		var frame pulp.WSFrame
		if err := msgpack.Unmarshal(ev.Payload, &frame); err != nil {
			return err
		}
		wsConnsMu.Lock()
		state, ok := wsConns[frame.ConnID]
		wsConnsMu.Unlock()
		if !ok {
			return nil
		}
		route := e.findWSRoute(state.path)
		if route == nil || route.handlers.OnFrame == nil {
			return nil
		}
		c := &WSContext{
			ConnID:  frame.ConnID,
			Path:    state.path,
			OpCode:  frame.OpCode,
			Payload: frame.Payload,
			Keys:    state.keys,
		}
		route.handlers.OnFrame(c)
	case pulp.EventWSClose:
		var closeEv pulp.WSClose
		if err := msgpack.Unmarshal(ev.Payload, &closeEv); err != nil {
			return err
		}
		wsConnsMu.Lock()
		state, ok := wsConns[closeEv.ConnID]
		if ok {
			delete(wsConns, closeEv.ConnID)
		}
		wsConnsMu.Unlock()
		if !ok {
			return nil
		}
		route := e.findWSRoute(state.path)
		if route == nil || route.handlers.OnClose == nil {
			return nil
		}
		c := &WSContext{
			ConnID: closeEv.ConnID,
			Path:   state.path,
			Code:   closeEv.Code,
			Reason: closeEv.Reason,
			Keys:   state.keys,
		}
		route.handlers.OnClose(c)
	}
	return nil
}

func (e *Engine) findWSRoute(path string) *wsRoute {
	for i := range e.wsRoutes { // exact first
		if e.wsRoutes[i].path == path {
			return &e.wsRoutes[i]
		}
	}
	for i := range e.wsRoutes { // then pattern (:param / *catchall, e.g. relay)
		if matchPattern(e.wsRoutes[i].path, path) {
			return &e.wsRoutes[i]
		}
	}
	return nil
}
