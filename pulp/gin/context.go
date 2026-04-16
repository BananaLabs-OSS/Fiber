package gin

import (
	"fmt"
	"strings"

	"github.com/BananaLabs-OSS/Fiber/pulp"
)

// Context mirrors the subset of gin.Context that handler code typically
// uses. The intent is source compatibility, not binary compatibility —
// existing handlers that are `func(c *gin.Context)` only need their
// import and parameter type changed to compile against pulpgin.
type Context struct {
	req       pulp.HTTPRequest
	params    map[string]string
	status    uint32
	headers   map[string]string
	body      []byte
	responded bool
}

// Request returns the raw decoded HTTPRequest the host delivered. Use
// when you need fields Gin's Context does not expose (the raw request
// ID, for instance).
func (c *Context) Request() pulp.HTTPRequest { return c.req }

// Param returns the path parameter captured at key. Returns empty
// string if the param does not exist.
func (c *Context) Param(key string) string {
	if c.params == nil {
		return ""
	}
	return c.params[key]
}

// Query returns the first value of the named query string parameter,
// or empty if absent.
func (c *Context) Query(key string) string {
	if c.req.Query == nil {
		return ""
	}
	return c.req.Query[key]
}

// DefaultQuery returns the query param or defaultValue if missing.
func (c *Context) DefaultQuery(key, defaultValue string) string {
	if v, ok := c.req.Query[key]; ok && v != "" {
		return v
	}
	return defaultValue
}

// GetHeader reads an inbound header. Gin is case-insensitive for
// headers; here we try the exact key first then a canonical form.
func (c *Context) GetHeader(key string) string {
	if c.req.Headers == nil {
		return ""
	}
	if v, ok := c.req.Headers[key]; ok {
		return v
	}
	// Try case-insensitive lookup.
	lower := strings.ToLower(key)
	for k, v := range c.req.Headers {
		if strings.ToLower(k) == lower {
			return v
		}
	}
	return ""
}

// Header sets an outbound response header.
func (c *Context) Header(key, value string) {
	if c.headers == nil {
		c.headers = map[string]string{}
	}
	c.headers[key] = value
}

// Status sets the HTTP status code for the response.
func (c *Context) Status(code int) {
	c.status = uint32(code)
}

// BindJSON unmarshals the request body as JSON into v. Returns the
// json package's error unchanged.
func (c *Context) BindJSON(v any) error {
	return jsonUnmarshal(c.req.Body, v)
}

// ShouldBindJSON is Gin's alias for BindJSON. Both exist because
// different handler styles pick different names.
func (c *Context) ShouldBindJSON(v any) error {
	return c.BindJSON(v)
}

// JSON writes status + a JSON-encoded body, sending the response.
func (c *Context) JSON(status int, obj any) {
	b, err := jsonMarshal(obj)
	if err != nil {
		c.String(500, "marshal error: %v", err)
		return
	}
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.status = uint32(status)
	c.body = b
	c.flush()
}

// String writes status + a formatted plaintext body, sending the response.
func (c *Context) String(status int, format string, values ...any) {
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.status = uint32(status)
	c.body = []byte(fmt.Sprintf(format, values...))
	c.flush()
}

// Data writes status + a raw body with the given Content-Type.
func (c *Context) Data(status int, contentType string, data []byte) {
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}
	c.status = uint32(status)
	c.body = data
	c.flush()
}

// AbortWithStatus sends an empty response with the given status and
// stops further handler logic (in Gin this aborts middleware — here it
// just ends the response since we do not yet run middleware chains).
func (c *Context) AbortWithStatus(status int) {
	c.status = uint32(status)
	c.body = nil
	c.flush()
}

// AbortWithStatusJSON sends status + a JSON body and stops.
func (c *Context) AbortWithStatusJSON(status int, obj any) {
	c.JSON(status, obj)
}

// Body returns the raw inbound request body.
func (c *Context) Body() []byte { return c.req.Body }

// ContentType returns the inbound Content-Type header.
func (c *Context) ContentType() string { return c.GetHeader("Content-Type") }

// populateParams extracts :param segments from pattern, matching them
// against the request path. The router has already verified that
// matchPattern returned true, so lengths align.
func (c *Context) populateParams(pattern, path string) {
	patternParts := strings.Split(strings.TrimPrefix(pattern, "/"), "/")
	pathParts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	c.params = map[string]string{}
	for i, p := range patternParts {
		if strings.HasPrefix(p, ":") {
			c.params[strings.TrimPrefix(p, ":")] = pathParts[i]
		}
	}
}

// flush hands the accumulated response to the host. Safe to call
// multiple times — subsequent calls are no-ops after the first.
func (c *Context) flush() {
	if c.responded {
		return
	}
	c.responded = true
	_ = pulp.HTTP.Respond(pulp.HTTPResponse{
		ID:      c.req.ID,
		Status:  c.status,
		Headers: c.headers,
		Body:    c.body,
	})
}
