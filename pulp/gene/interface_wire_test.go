package gene

import (
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestHTTPRequestClientIPIsAdditiveAndOptional(t *testing.T) {
	legacy, err := msgpack.Marshal(map[string]any{
		"method": "POST",
		"path":   "/api/checkout",
		"body":   []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded HTTPRequest
	if err := msgpack.Unmarshal(legacy, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ClientIP != "" || decoded.Method != "POST" || decoded.Path != "/api/checkout" {
		t.Fatalf("legacy request decode = %#v", decoded)
	}

	wire, err := msgpack.Marshal(HTTPRequest{
		Method:   "POST",
		Path:     "/api/checkout",
		ClientIP: "203.0.113.42",
	})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := msgpack.Unmarshal(wire, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["client_ip"] != "203.0.113.42" {
		t.Fatalf("client_ip wire = %#v", fields["client_ip"])
	}

	withoutIP, err := msgpack.Marshal(HTTPRequest{Method: "GET", Path: "/api/tiers"})
	if err != nil {
		t.Fatal(err)
	}
	fields = nil
	if err := msgpack.Unmarshal(withoutIP, &fields); err != nil {
		t.Fatal(err)
	}
	if _, exists := fields["client_ip"]; exists {
		t.Fatalf("empty client_ip changed legacy wire: %#v", fields)
	}
}
