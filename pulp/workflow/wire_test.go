package workflow

import (
	"strings"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestDispatchWireRoundTrip(t *testing.T) {
	request := DispatchRequest{
		Event: "evolution.gene.route.requested.v1",
		Payload: map[string]any{
			"gene": "sessions",
			"path": "/api/tiers",
		},
	}
	encoded, err := msgpack.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var decoded DispatchRequest
	if err := msgpack.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if decoded.Event != request.Event {
		t.Fatalf("event = %q, want %q", decoded.Event, request.Event)
	}
	payload, ok := decoded.Payload.(map[string]any)
	if !ok || payload["gene"] != "sessions" || payload["path"] != "/api/tiers" {
		t.Fatalf("payload = %#v", decoded.Payload)
	}

	result := DispatchResult{
		Value: map[string]any{"status": int64(200), "body": "ok"},
		Commands: []Action{{
			Name:    "orders.schedule.v1",
			Payload: map[string]any{"order_id": "order-1"},
		}},
		Events: []Action{{
			Name:    "voucher.scheduled.v1",
			Payload: map[string]any{"order_id": "order-1"},
		}},
	}
	encoded, err = msgpack.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var decodedResult DispatchResult
	if err := msgpack.Unmarshal(encoded, &decodedResult); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if err := decodedResult.Validate(); err != nil {
		t.Fatalf("validate result: %v", err)
	}
	if len(decodedResult.Commands) != 1 || decodedResult.Commands[0].Name != "orders.schedule.v1" {
		t.Fatalf("commands = %#v", decodedResult.Commands)
	}
	if len(decodedResult.Events) != 1 || decodedResult.Events[0].Name != "voucher.scheduled.v1" {
		t.Fatalf("events = %#v", decodedResult.Events)
	}
}

func TestDispatchValidation(t *testing.T) {
	for _, request := range []DispatchRequest{
		{},
		{Event: " surrounding-space "},
		{Event: "bad\nname"},
		{Event: strings.Repeat("x", maxNameLength+1)},
	} {
		if err := request.Validate(); err == nil {
			t.Fatalf("Validate(%q) succeeded", request.Event)
		}
	}
	if err := (DispatchRequest{Event: "health"}).Validate(); err != nil {
		t.Fatalf("valid request: %v", err)
	}

	result := DispatchResult{Commands: []Action{{Name: ""}}}
	if err := result.Validate(); err == nil || !strings.Contains(err.Error(), "command 0") {
		t.Fatalf("result validation error = %v", err)
	}
	result = DispatchResult{Events: []Action{{Name: "bad\tname"}}}
	if err := result.Validate(); err == nil || !strings.Contains(err.Error(), "event 0") {
		t.Fatalf("result validation error = %v", err)
	}
}

func TestDecodeValue(t *testing.T) {
	type response struct {
		Status uint32 `msgpack:"status"`
		Body   []byte `msgpack:"body"`
	}
	value, err := DecodeValue[response](DispatchResult{
		Value: map[string]any{
			"status": int64(200),
			"body":   "ready",
		},
	})
	if err != nil {
		t.Fatalf("DecodeValue: %v", err)
	}
	if value.Status != 200 || string(value.Body) != "ready" {
		t.Fatalf("value = %#v", value)
	}

	if _, err := DecodeValue[response](DispatchResult{}); err == nil {
		t.Fatal("DecodeValue accepted a missing value")
	}
}
