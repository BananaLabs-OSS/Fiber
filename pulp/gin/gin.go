// Package gin is a Gin-compatible HTTP router that runs inside a Pulp
// cell. Existing Gin handler code compiles unchanged — only the
// bootstrap swaps (gin.Default() → pulpgin.New()). Handlers receive a
// *pulpgin.Context with Gin-identical methods.
//
// Typical usage:
//
//	func main() {}
//
//	func init() {
//		r := pulpgin.New()
//		r.GET("/users/:id", getUser)
//		r.POST("/users", createUser)
//		r.Run()
//	}
//
//	func getUser(c *pulpgin.Context) {
//		id := c.Param("id")
//		c.JSON(200, pulpgin.H{"id": id})
//	}
package gin

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

// HandlerFunc mirrors gin.HandlerFunc.
type HandlerFunc func(c *Context)

// H is the standard Gin shortcut for a map[string]any used in JSON
// responses. Exact alias so existing gin.H literals compile unchanged
// after a simple import rewrite.
type H map[string]any

// Engine is the router. It owns the full route table and the root
// middleware stack; Group creates a RouterGroup that inherits the
// current middleware and adds a path prefix. The engine also owns the
// WebSocket route table; cells use Engine.WS to register ws.* routes.
type Engine struct {
	routes     []route
	wsRoutes   []wsRoute
	middleware []HandlerFunc
}

// RouterGroup is a scope attached to a shared prefix + middleware
// stack. Obtained via Engine.Group. Further Group calls on a group
// nest deeper. Mirrors gin.RouterGroup for the methods services
// typically call.
type RouterGroup struct {
	engine     *Engine
	prefix     string
	middleware []HandlerFunc
}

type route struct {
	method     string
	pattern    string
	handlers   []HandlerFunc
}

// New returns a fresh Engine. Multiple engines are not supported — the
// last one to call Run wins the pulp.OnStep registration.
func New() *Engine {
	return &Engine{}
}

// Default is an alias for New, preserved for drop-in Gin compatibility.
func Default() *Engine {
	return New()
}

// Use appends middleware to the engine's global stack. Every route
// registered after Use is called runs through these middleware in
// order before the handler.
func (e *Engine) Use(middleware ...HandlerFunc) *Engine {
	e.middleware = append(e.middleware, middleware...)
	return e
}

// Group returns a RouterGroup that prepends prefix to every route
// registered through it and inherits the engine's current middleware
// stack. Gin pattern: r.Group("/api").Use(auth).GET("/users", h).
func (e *Engine) Group(prefix string, middleware ...HandlerFunc) *RouterGroup {
	inherited := append([]HandlerFunc{}, e.middleware...)
	inherited = append(inherited, middleware...)
	return &RouterGroup{
		engine:     e,
		prefix:     normalizePrefix(prefix),
		middleware: inherited,
	}
}

// Use adds middleware to this group. Runs after anything inherited
// from the parent engine/group.
func (g *RouterGroup) Use(middleware ...HandlerFunc) *RouterGroup {
	g.middleware = append(g.middleware, middleware...)
	return g
}

// Group creates a nested RouterGroup. The resulting prefix is the
// parent's prefix joined with the child's prefix, with slashes
// normalized so ("/") + ("/x") becomes "/x" not "//x".
func (g *RouterGroup) Group(prefix string, middleware ...HandlerFunc) *RouterGroup {
	inherited := append([]HandlerFunc{}, g.middleware...)
	inherited = append(inherited, middleware...)
	return &RouterGroup{
		engine:     g.engine,
		prefix:     joinPath(g.prefix, prefix),
		middleware: inherited,
	}
}

// GET registers a GET route on the engine. Pattern segments prefixed
// with ":" are captured as path params.
func (e *Engine) GET(pattern string, handler HandlerFunc) *Engine {
	return e.Handle("GET", pattern, handler)
}

// POST registers a POST route.
func (e *Engine) POST(pattern string, handler HandlerFunc) *Engine {
	return e.Handle("POST", pattern, handler)
}

// PUT registers a PUT route.
func (e *Engine) PUT(pattern string, handler HandlerFunc) *Engine {
	return e.Handle("PUT", pattern, handler)
}

// DELETE registers a DELETE route.
func (e *Engine) DELETE(pattern string, handler HandlerFunc) *Engine {
	return e.Handle("DELETE", pattern, handler)
}

// PATCH registers a PATCH route.
func (e *Engine) PATCH(pattern string, handler HandlerFunc) *Engine {
	return e.Handle("PATCH", pattern, handler)
}

// Handle registers a route for an arbitrary method at the engine's
// root. Middleware already installed via Use runs before the handler.
func (e *Engine) Handle(method, pattern string, handler HandlerFunc) *Engine {
	chain := append([]HandlerFunc{}, e.middleware...)
	chain = append(chain, handler)
	e.routes = append(e.routes, route{
		method:   strings.ToUpper(method),
		pattern:  pattern,
		handlers: chain,
	})
	return e
}

// GET registers a GET route on this group. The full pattern becomes
// group.prefix + pattern.
func (g *RouterGroup) GET(pattern string, handler HandlerFunc) *RouterGroup {
	return g.Handle("GET", pattern, handler)
}

// POST registers a POST route on this group.
func (g *RouterGroup) POST(pattern string, handler HandlerFunc) *RouterGroup {
	return g.Handle("POST", pattern, handler)
}

// PUT registers a PUT route on this group.
func (g *RouterGroup) PUT(pattern string, handler HandlerFunc) *RouterGroup {
	return g.Handle("PUT", pattern, handler)
}

// DELETE registers a DELETE route on this group.
func (g *RouterGroup) DELETE(pattern string, handler HandlerFunc) *RouterGroup {
	return g.Handle("DELETE", pattern, handler)
}

// PATCH registers a PATCH route on this group.
func (g *RouterGroup) PATCH(pattern string, handler HandlerFunc) *RouterGroup {
	return g.Handle("PATCH", pattern, handler)
}

// Handle registers a route for an arbitrary method on this group.
// Chain order is: engine middleware → group middleware → handler.
func (g *RouterGroup) Handle(method, pattern string, handler HandlerFunc) *RouterGroup {
	chain := append([]HandlerFunc{}, g.middleware...)
	chain = append(chain, handler)
	g.engine.routes = append(g.engine.routes, route{
		method:   strings.ToUpper(method),
		pattern:  joinPath(g.prefix, pattern),
		handlers: chain,
	})
	return g
}

// Run registers every declared route with the host, installs a
// pulp.OnStep dispatcher, and returns. Call this from your init() or
// pulp.OnInit callback — it does not block.
//
// Cells that need to run periodic work (polling, metrics) alongside
// HTTP dispatch should call RegisterRoutes + pulp.OnStep themselves,
// invoking Dispatch for HTTP events. See Dispatch for the idiom.
func (e *Engine) Run(addr ...string) error {
	if err := e.RegisterRoutes(); err != nil {
		return err
	}
	pulp.OnStep(e.Dispatch)
	return nil
}

// RegisterRoutes declares every route — both HTTP and WebSocket —
// with the host without installing a pulp.OnStep handler. Use when
// you compose a custom step handler that dispatches events via
// Dispatch.
//
// Routes are sorted by specificity before registration so static
// patterns (no ":param" segments) win over parametric ones during
// dispatch — for example /presence/count wins over /presence/:userId
// even when the latter was declared first.
func (e *Engine) RegisterRoutes() error {
	sort.SliceStable(e.routes, func(i, j int) bool {
		return paramCount(e.routes[i].pattern) < paramCount(e.routes[j].pattern)
	})
	for _, r := range e.routes {
		if err := pulp.HTTP.Register(r.method, r.pattern); err != nil {
			return fmt.Errorf("register %s %s: %w", r.method, r.pattern, err)
		}
	}
	for _, r := range e.wsRoutes {
		if err := pulp.WS.Register(r.path); err != nil {
			return fmt.Errorf("register ws %s: %w", r.path, err)
		}
	}
	return nil
}

// paramCount returns the number of ":param" segments in pattern.
// Used only to order routes by specificity during RegisterRoutes.
func paramCount(pattern string) int {
	n := 0
	for _, seg := range strings.Split(pattern, "/") {
		if strings.HasPrefix(seg, ":") {
			n++
		}
	}
	return n
}

// Dispatch routes a single step event to the matching handler — HTTP
// or WebSocket. Returns nil for events the engine does not own, so
// it is safe to call unconditionally from a composed OnStep. Cells
// that need to run periodic work wrap Dispatch in a closure:
//
//	r := pulpgin.New()
//	r.GET(...)
//	r.WS(...)
//	_ = r.RegisterRoutes()
//	pulp.OnStep(func(ev pulp.StepEvent) error {
//		myPeriodicWork(ev.WallTime)
//		return r.Dispatch(ev)
//	})
func (e *Engine) Dispatch(ev pulp.StepEvent) error {
	switch ev.Kind {
	case pulp.EventHTTPRequest:
		return e.dispatch(ev)
	case pulp.EventWSOpen, pulp.EventWSFrame, pulp.EventWSClose:
		return e.dispatchWS(ev)
	}
	return nil
}

// dispatch is installed as the cell's step handler. It receives every
// event the host delivers, filters for HTTP requests, finds a matching
// route, and invokes the handler with a populated Context.
func (e *Engine) dispatch(ev pulp.StepEvent) error {
	if ev.Kind != pulp.EventHTTPRequest {
		return nil
	}
	var req pulp.HTTPRequest
	if err := msgpack.Unmarshal(ev.Payload, &req); err != nil {
		return fmt.Errorf("decode HTTPRequest: %w", err)
	}

	for _, r := range e.routes {
		if r.method != req.Method {
			continue
		}
		if !matchPattern(r.pattern, req.Path) {
			continue
		}
		c := &Context{req: req, handlers: r.handlers}
		c.populateParams(r.pattern, req.Path)
		c.Next()
		if !c.responded {
			// Handler never called a response method — send empty 200
			// so the client is not left hanging. Matches Gin's default
			// when a handler returns without writing.
			c.Status(200)
			c.flush()
		}
		return nil
	}

	// CORS preflight fallback: if the request is OPTIONS and ANY route
	// exists at this path (any method), reuse that route's handler chain
	// so engine middleware (CORS) runs and returns the preflight 204.
	// Native Gin auto-handles OPTIONS for paths with other methods; the
	// Pulp port doesn't, so cells that rely on a global CORS middleware
	// would otherwise 404 on every preflight.
	if req.Method == "OPTIONS" {
		for _, r := range e.routes {
			if !matchPattern(r.pattern, req.Path) {
				continue
			}
			c := &Context{req: req, handlers: r.handlers}
			c.populateParams(r.pattern, req.Path)
			c.Next()
			if !c.responded {
				c.Status(204)
				c.flush()
			}
			return nil
		}
	}

	// No route matched. Respond 404 with the exact shape Gin's default
	// NoRoute emits (plain "text/plain" content type, "404 page not
	// found" body), so parity tests comparing against a native Gin
	// server pass without special-casing the 404 body.
	return pulp.HTTP.Respond(pulp.HTTPResponse{
		ID:      req.ID,
		Status:  404,
		Headers: map[string]string{"Content-Type": "text/plain"},
		Body:    []byte("404 page not found"),
	})
}

func matchPattern(pattern, path string) bool {
	patternParts := strings.Split(strings.TrimPrefix(pattern, "/"), "/")
	pathParts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(patternParts) != len(pathParts) {
		return false
	}
	for i, p := range patternParts {
		if strings.HasPrefix(p, ":") {
			continue
		}
		if p != pathParts[i] {
			return false
		}
	}
	return true
}

// JSON-encoded literal shortcuts used for HandlerFunc.BindJSON. Alias
// to keep gin code's import-less usage intact.
var (
	jsonMarshal   = json.Marshal
	jsonUnmarshal = json.Unmarshal
)

// normalizePrefix coerces a group prefix to canonical form: empty or
// "/" becomes "" (so joining with a pattern gives the pattern back
// verbatim); any other non-slash-prefixed string gets a leading slash;
// trailing slashes are stripped. Keeps the routing table's patterns
// free of accidental "//foo" double slashes.
func normalizePrefix(p string) string {
	if p == "" || p == "/" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimRight(p, "/")
}

// joinPath glues a normalized group prefix to a per-route pattern so
// the final registered route has no doubled or missing slashes.
// "/foo" + "/bar" -> "/foo/bar"; "" + "/bar" -> "/bar"; "/foo" + "" -> "/foo".
func joinPath(prefix, pattern string) string {
	prefix = normalizePrefix(prefix)
	if pattern == "" {
		if prefix == "" {
			return "/"
		}
		return prefix
	}
	if !strings.HasPrefix(pattern, "/") {
		pattern = "/" + pattern
	}
	return prefix + pattern
}
