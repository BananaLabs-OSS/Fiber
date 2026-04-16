// Package gin is a Gin-compatible HTTP router that runs inside a Pulp
// plugin. Existing Gin handler code compiles unchanged — only the
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

// Engine is the router. It owns the map of registered routes and
// translates each step event that arrives via pulp.OnStep into a
// handler call.
type Engine struct {
	routes []route
}

type route struct {
	method  string
	pattern string
	handler HandlerFunc
}

// New returns a fresh Engine. Multiple engines are not supported — the
// last one to call Run wins the pulp.OnStep registration.
func New() *Engine {
	return &Engine{}
}

// Default is an alias for New for drop-in compatibility with Gin code.
// Gin's middleware stack has no equivalent yet — this simply returns
// New().
func Default() *Engine {
	return New()
}

// GET registers a GET route. Pattern segments prefixed with ":" are
// captured as path params.
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

// Handle registers a route for an arbitrary method.
func (e *Engine) Handle(method, pattern string, handler HandlerFunc) *Engine {
	e.routes = append(e.routes, route{
		method:  strings.ToUpper(method),
		pattern: pattern,
		handler: handler,
	})
	return e
}

// Run registers every declared route with the host, installs a
// pulp.OnStep dispatcher, and returns. Call this from your init() or
// pulp.OnInit callback — it does not block.
func (e *Engine) Run(addr ...string) error {
	for _, r := range e.routes {
		if err := pulp.HTTP.Register(r.method, r.pattern); err != nil {
			return fmt.Errorf("register %s %s: %w", r.method, r.pattern, err)
		}
	}
	pulp.OnStep(e.dispatch)
	return nil
}

// dispatch is installed as the plugin's step handler. It receives every
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
		c := &Context{req: req}
		c.populateParams(r.pattern, req.Path)
		r.handler(c)
		if !c.responded {
			// Handler never called a response method — send empty 200
			// so the client is not left hanging. Matches Gin's default
			// when a handler returns without writing.
			c.Status(200)
			c.flush()
		}
		return nil
	}

	// No route matched. Respond 404 via the host.
	return pulp.HTTP.Respond(pulp.HTTPResponse{
		ID:      req.ID,
		Status:  404,
		Headers: map[string]string{"Content-Type": "text/plain; charset=utf-8"},
		Body:    []byte("404 not found\n"),
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
