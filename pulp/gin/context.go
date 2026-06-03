package gin

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"strconv"
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
	cookies   []string // one formatted Set-Cookie value per entry
	body      []byte
	responded bool

	// sameSite defaults to 0 (http.SameSiteDefaultMode). SetSameSite
	// mutates it; the next SetCookie call consumes it.
	sameSite http.SameSite

	// handlers is the ordered middleware+handler chain for this route.
	// Next() walks it; handlers read index to know where they are.
	handlers []HandlerFunc
	index    int
	aborted  bool

	// keys is Gin's per-request scratch space, populated by middleware
	// (e.g. JWTAuth writes "account_id" here) and read by handlers.
	keys map[string]any

	// trustedProxies is the engine's configured trusted-proxy CIDR set,
	// copied in at dispatch. ClientIP() honors forwarded headers only
	// when the immediate peer (RemoteAddr) is within this set; nil means
	// trust nothing. See Engine.SetTrustedProxies.
	trustedProxies []*net.IPNet
}

// Next advances through the middleware/handler chain. Middleware call
// Next() to run the rest of the chain, then perform post-processing
// when it returns. Call sites that never invoke Next for a request
// abort the chain — use Abort or one of the AbortWith* methods.
func (c *Context) Next() {
	c.index++
	for c.index <= len(c.handlers) && !c.aborted {
		c.handlers[c.index-1](c)
		c.index++
	}
}

// Abort stops further handler execution. Call from a middleware that
// has already written a response (e.g. JWTAuth rejecting a bad token).
func (c *Context) Abort() {
	c.aborted = true
}

// IsAborted reports whether the request chain has been aborted.
func (c *Context) IsAborted() bool { return c.aborted }

// Set stores value under key in the per-request scratch space. Used
// by middleware to pass data (account IDs, parsed tokens) to handlers.
func (c *Context) Set(key string, value any) {
	if c.keys == nil {
		c.keys = map[string]any{}
	}
	c.keys[key] = value
}

// Get retrieves a value set by middleware. Returns (value, true) on
// hit, (nil, false) on miss — same semantics as gin.Context.Get.
func (c *Context) Get(key string) (any, bool) {
	if c.keys == nil {
		return nil, false
	}
	v, ok := c.keys[key]
	return v, ok
}

// GetString is the typed convenience wrapper. Returns zero value when
// the key is missing or wrong type.
func (c *Context) GetString(key string) string {
	v, ok := c.Get(key)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// MustGet is like Get but panics when the key is missing. Rarely used
// in practice; included for Gin parity.
func (c *Context) MustGet(key string) any {
	v, ok := c.Get(key)
	if !ok {
		panic("key not found: " + key)
	}
	return v
}

// Request returns the raw decoded HTTPRequest the host delivered. Use
// when you need fields Gin's Context does not expose (the raw request
// ID, for instance).
func (c *Context) Request() pulp.HTTPRequest { return c.req }

// Ctx returns the per-request Go context. Gin handler code typically
// calls c.Request.Context() for this — the pulpgin equivalent is
// c.Ctx().
//
// Limitation: this currently returns context.Background(). Plumbing
// host request-cancellation through the ABI requires adding a
// cancel-token channel to StepEvent + periodic host→cell notify,
// which is a cross-repo ABI change. Until then, long-running cell
// handlers cannot observe client disconnect via ctx.Done(). The
// cell step loop is single-threaded so Go-level goroutine
// cancellation inside a handler is a no-op anyway — this only
// matters for HTTP clients (pulp.HTTP.Fetch) that honor the passed
// context, where passing Background means no cancellation on client
// disconnect. If you need that, use pulp.HTTPFetchRequest.Timeout.
func (c *Context) Ctx() context.Context {
	return context.Background()
}

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

// ClientIP returns the client's IP address.
//
// SECURITY: forwarded proxy headers (CF-Connecting-IP, X-Forwarded-For,
// X-Real-Ip, RFC 7239 Forwarded) are client-controllable and trivially
// spoofable — any caller can set CF-Connecting-IP: 1.2.3.4. ClientIP()
// therefore trusts those headers ONLY when the immediate peer
// (host-observed RemoteAddr) is within the engine's configured
// trusted-proxy set (see Engine.SetTrustedProxies / Engine.TrustCloudflare).
//
//   - Default (no trusted proxies configured): forwarded headers are
//     ignored entirely and ClientIP() returns the real peer RemoteAddr.
//     This is the safe default — a client talking to the cell origin
//     directly cannot forge its IP.
//   - Behind Cloudflare (TrustCloudflare): when RemoteAddr is a Cloudflare
//     edge, CF-Connecting-IP is honored as the authoritative client IP;
//     the raw X-Forwarded-For first hop and True-Client-IP are NOT trusted
//     ahead of it, since on a CF-fronted origin only CF-Connecting-IP is
//     set by the edge.
//
// Use this for per-IP rate limits, blocklists, geo-gating, and audit
// fields — it is only as trustworthy as the trusted-proxy configuration.
// Returns empty string only when no address is available at all.
func (c *Context) ClientIP() string {
	peer := stripPort(c.req.RemoteAddr)

	// Only consult forwarded headers when the immediate peer is a trusted
	// proxy. Otherwise the headers are attacker-supplied; return the peer.
	if !c.peerIsTrusted(peer) {
		return peer
	}

	// Trusted edge. Prefer Cloudflare's authoritative header, then the
	// conventional proxy headers (leftmost X-Forwarded-For hop, X-Real-Ip,
	// RFC 7239 Forwarded). True-Client-IP is deliberately not honored: it
	// is an Enterprise-only CF feature that mirrors CF-Connecting-IP, and
	// trusting it ahead of CF-Connecting-IP just widens the spoof surface.
	if ip := c.GetHeader("CF-Connecting-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		if idx := strings.IndexByte(xff, ','); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if rip := c.GetHeader("X-Real-Ip"); rip != "" {
		return strings.TrimSpace(rip)
	}
	if fwd := c.GetHeader("Forwarded"); fwd != "" {
		for _, part := range strings.Split(fwd, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToLower(part), "for=") {
				v := strings.Trim(part[4:], `"`)
				v = strings.TrimPrefix(v, "[")
				if idx := strings.LastIndexByte(v, ']'); idx != -1 {
					v = v[:idx]
				}
				if v != "" {
					return v
				}
			}
		}
	}
	// Trusted peer but no forwarded header present — return the peer.
	return peer
}

// peerIsTrusted reports whether the immediate-peer IP is within the
// engine's configured trusted-proxy set. A nil/empty set trusts nothing.
func (c *Context) peerIsTrusted(peer string) bool {
	if len(c.trustedProxies) == 0 || peer == "" {
		return false
	}
	ip := net.ParseIP(peer)
	if ip == nil {
		return false
	}
	for _, n := range c.trustedProxies {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// stripPort returns the host portion of a "host:port" RemoteAddr,
// handling IPv6 bracketed form ("[::1]:1234"). Returns "" for empty
// input. A bare IP with no port is returned unchanged.
func stripPort(addr string) string {
	if addr == "" {
		return ""
	}
	// IPv6 bracketed form: [::1]:1234
	if strings.HasPrefix(addr, "[") {
		if end := strings.LastIndexByte(addr, ']'); end != -1 {
			return addr[1:end]
		}
	}
	if idx := strings.LastIndexByte(addr, ':'); idx != -1 {
		// For plain IPv4 "a.b.c.d:port" the last colon is the port
		// separator. A bare IPv6 literal (multiple colons, no brackets)
		// has no port, so leave it untouched.
		if !strings.Contains(addr[:idx], ":") {
			return addr[:idx]
		}
	}
	return addr
}

// Header sets an outbound response header.
func (c *Context) Header(key, value string) {
	if c.headers == nil {
		c.headers = map[string]string{}
	}
	c.headers[key] = value
}

// Status sets the HTTP status code and sends an empty response.
func (c *Context) Status(code int) {
	c.status = uint32(code)
	c.flush()
}

// BindJSON unmarshals the request body as JSON into v. Returns the
// json package's error unchanged.
func (c *Context) BindJSON(v any) error {
	if err := jsonUnmarshal(c.req.Body, v); err != nil {
		return err
	}
	return validateBinding(v)
}

// ShouldBindJSON is Gin's alias for BindJSON. Both exist because
// different handler styles pick different names.
func (c *Context) ShouldBindJSON(v any) error {
	return c.BindJSON(v)
}

// validateBinding applies a minimal subset of go-playground/validator
// rules — specifically `binding:"required"` — so cell handlers that
// ported from native Gin with `binding:"required"` on their request
// structs get the same 400-with-validation-error behavior instead of
// silently accepting missing fields.
//
// Supported tags: "required". Other tags are accepted but not
// enforced. Error format mirrors Gin's validator output so parity
// tests pass without special-casing:
//
//	Key: 'LoginRequest.Email' Error:Field validation for 'Email' failed on the 'required' tag
func validateBinding(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return nil
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return nil
	}
	rt := rv.Type()
	structName := rt.Name()
	var messages []string
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("binding")
		if tag == "" || tag == "-" {
			continue
		}
		for _, rule := range strings.Split(tag, ",") {
			rule = strings.TrimSpace(rule)
			if rule != "required" {
				continue
			}
			fv := rv.Field(i)
			if fv.IsZero() {
				messages = append(messages, fmt.Sprintf(
					"Key: '%s.%s' Error:Field validation for '%s' failed on the 'required' tag",
					structName, field.Name, field.Name,
				))
			}
		}
	}
	if len(messages) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(messages, "\n"))
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

// Redirect sends an HTTP redirect response — sets the Location header
// and status, typically 302 or 307. Mirrors gin.Context.Redirect.
func (c *Context) Redirect(status int, url string) {
	c.Header("Location", url)
	c.status = uint32(status)
	c.body = nil
	c.flush()
}

// AbortWithStatus sends an empty response with the given status and
// aborts the handler chain.
func (c *Context) AbortWithStatus(status int) {
	c.aborted = true
	c.status = uint32(status)
	c.body = nil
	c.flush()
}

// AbortWithStatusJSON sends status + a JSON body and aborts the chain.
func (c *Context) AbortWithStatusJSON(status int, obj any) {
	c.aborted = true
	c.JSON(status, obj)
}

// Body returns the raw inbound request body.
func (c *Context) Body() []byte { return c.req.Body }

// Cookie returns the value of the named cookie from the request.
// Returns ("", error) if the cookie is not present.
func (c *Context) Cookie(name string) (string, error) {
	header := c.GetHeader("Cookie")
	if header == "" {
		return "", fmt.Errorf("cookie %q not found", name)
	}
	for _, part := range strings.Split(header, ";") {
		part = strings.TrimSpace(part)
		if i := strings.IndexByte(part, '='); i > 0 {
			if part[:i] == name {
				return part[i+1:], nil
			}
		}
	}
	return "", fmt.Errorf("cookie %q not found", name)
}

// SetCookie adds a Set-Cookie header to the response. Mirrors gin's
// SetCookie(name, value, maxAge, path, domain, secure, httpOnly).
// Multiple SetCookie calls in the same response all land on the wire —
// the host emits one Set-Cookie header per entry.
//
// The SameSite attribute is taken from the last SetSameSite call (or
// http.SameSiteDefaultMode if none). Each SetCookie consumes and then
// resets the sameSite flag, matching Gin's behavior.
func (c *Context) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	cookie := name + "=" + value
	if path != "" {
		cookie += "; Path=" + path
	}
	if domain != "" {
		cookie += "; Domain=" + domain
	}
	if maxAge > 0 {
		cookie += "; Max-Age=" + fmt.Sprintf("%d", maxAge)
	} else if maxAge < 0 {
		cookie += "; Max-Age=0"
	}
	if secure {
		cookie += "; Secure"
	}
	if httpOnly {
		cookie += "; HttpOnly"
	}
	switch c.sameSite {
	case http.SameSiteLaxMode:
		cookie += "; SameSite=Lax"
	case http.SameSiteStrictMode:
		cookie += "; SameSite=Strict"
	case http.SameSiteNoneMode:
		cookie += "; SameSite=None"
	}
	c.sameSite = http.SameSiteDefaultMode
	c.cookies = append(c.cookies, cookie)
}

// SetSameSite sets the SameSite attribute applied to the next SetCookie
// call. Matches Gin's SetSameSite — it affects only the immediately
// following SetCookie; a SetCookie without a prior SetSameSite emits
// no SameSite attribute (browsers default to Lax).
func (c *Context) SetSameSite(s http.SameSite) {
	c.sameSite = s
}

// PostForm returns the first value for the named form field from a
// application/x-www-form-urlencoded body. Values are URL-decoded so
// that "%40" becomes "@" and "+" becomes " ", matching net/http.
func (c *Context) PostForm(key string) string {
	values, err := url.ParseQuery(string(c.req.Body))
	if err != nil {
		return ""
	}
	return values.Get(key)
}

// ContentType returns the inbound Content-Type header.
// ContentType returns just the media-type portion of Content-Type,
// stripping any "; charset=..." or other parameters, matching Gin's
// filterFlags behavior. Cell handlers that compare against
// "application/json" will match regardless of charset suffix.
func (c *Context) ContentType() string {
	ct := c.GetHeader("Content-Type")
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.TrimSpace(ct)
}

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

// ShouldBind dispatches to a binder based on the request Content-Type:
// application/json → ShouldBindJSON; form-encoded (urlencoded or
// multipart) → ShouldBindForm; anything else falls back to JSON. Matches
// gin.Context.ShouldBind's contract for handler source compatibility.
func (c *Context) ShouldBind(obj any) error {
	ct := c.ContentType()
	if i := strings.IndexByte(ct, ';'); i != -1 {
		ct = ct[:i]
	}
	ct = strings.TrimSpace(strings.ToLower(ct))
	switch ct {
	case "application/json":
		return c.ShouldBindJSON(obj)
	case "application/x-www-form-urlencoded", "multipart/form-data":
		return c.ShouldBindForm(obj)
	default:
		return c.ShouldBindJSON(obj)
	}
}

// ShouldBindForm parses the request body as application/x-www-form-urlencoded
// and populates obj by `form:"key"` tag. Multipart bodies are not parsed —
// returns an explicit error in that case.
func (c *Context) ShouldBindForm(obj any) error {
	ct := c.ContentType()
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(ct)), "multipart/form-data") {
		return fmt.Errorf("form binding not implemented for multipart/form-data — use ShouldBindJSON")
	}
	values, err := url.ParseQuery(string(c.req.Body))
	if err != nil {
		return err
	}
	return bindValues(obj, values, "form")
}

// ShouldBindQuery populates obj from the request's URL query string via
// `form:"key"` tags. Mirrors gin.Context.ShouldBindQuery.
func (c *Context) ShouldBindQuery(obj any) error {
	values := url.Values{}
	for k, v := range c.req.Query {
		values.Set(k, v)
	}
	return bindValues(obj, values, "form")
}

// ShouldBindUri populates obj from path params via `uri:"key"` tags.
// Mirrors gin.Context.ShouldBindUri.
func (c *Context) ShouldBindUri(obj any) error {
	values := url.Values{}
	for k, v := range c.params {
		values.Set(k, v)
	}
	return bindValues(obj, values, "uri")
}

// bindValues walks obj (must be pointer to struct) and assigns each field
// from values using the given struct tag. Unsupported field types are
// skipped silently — matches gin's permissive binder.
func bindValues(obj any, values url.Values, tag string) error {
	rv := reflect.ValueOf(obj)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("bind target must be a non-nil pointer")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("bind target must point to a struct")
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}
		key := field.Tag.Get(tag)
		if key == "" {
			continue
		}
		if idx := strings.IndexByte(key, ','); idx != -1 {
			key = key[:idx]
		}
		if key == "" || key == "-" {
			continue
		}
		raw, ok := values[key]
		if !ok || len(raw) == 0 {
			continue
		}
		fv := rv.Field(i)
		if !fv.CanSet() {
			continue
		}
		switch fv.Kind() {
		case reflect.String:
			fv.SetString(raw[0])
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if n, err := strconv.ParseInt(raw[0], 10, 64); err == nil {
				fv.SetInt(n)
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if n, err := strconv.ParseUint(raw[0], 10, 64); err == nil {
				fv.SetUint(n)
			}
		case reflect.Float32, reflect.Float64:
			if f, err := strconv.ParseFloat(raw[0], 64); err == nil {
				fv.SetFloat(f)
			}
		case reflect.Bool:
			if b, err := strconv.ParseBool(raw[0]); err == nil {
				fv.SetBool(b)
			}
		case reflect.Slice:
			if fv.Type().Elem().Kind() == reflect.String {
				fv.Set(reflect.ValueOf(append([]string(nil), raw...)))
			}
		default:
			// Unsupported (nested struct, time.Time, etc.) — skip silently.
		}
	}
	return nil
}

// File reads filepath from the cell's scoped storage and writes it as
// the response body. Content-Type is inferred from the file extension.
// Missing files produce 404 (matches Gin); other errors produce 500.
func (c *Context) File(filepath_ string) {
	data, err := pulp.FS.Read(filepath_)
	if err != nil {
		if errors.Is(err, pulp.ErrNotFound) {
			c.String(404, "404 page not found")
			return
		}
		c.String(500, "file error: %v", err)
		return
	}
	ct := mime.TypeByExtension(filepath.Ext(filepath_))
	if ct == "" {
		ct = "application/octet-stream"
	}
	c.Data(200, ct, data)
}

// FileFromFS is a stub — WASM cells have no access to arbitrary
// http.FileSystem handles. Present only to satisfy handler code ported
// from a stock Gin service.
func (c *Context) FileFromFS(filepath_ string, fs http.FileSystem) {
	c.String(500, "FileFromFS not supported in WASM cells")
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
		Cookies: c.cookies,
		Body:    c.body,
	})
}
