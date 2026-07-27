package effect

import (
	"strings"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestHTTPProbeIntentBindsOwnerIdentityAndDerivesStableHostSelectors(t *testing.T) {
	command := validHTTPProbeCommandV1()
	intent, err := NewIntent(
		command.EffectID,
		KindHTTPProbeExecute,
		command.IdempotencyKey,
		command,
	)
	if err != nil {
		t.Fatal(err)
	}
	destination, err := HTTPProbeDestinationV1(command)
	if err != nil {
		t.Fatal(err)
	}
	fence, err := HTTPProbeFenceV1(command)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(destination, "status.website.") ||
		!strings.HasPrefix(fence, "status-probe.v1.") {
		t.Fatalf("derived destination/fence = %q/%q", destination, fence)
	}
	if err := intent.Validate(); err != nil {
		t.Fatal(err)
	}

	tampered := command
	tampered.URL = "https://sessions.gg/health"
	otherDestination, err := HTTPProbeDestinationV1(tampered)
	if err != nil {
		t.Fatal(err)
	}
	otherFence, err := HTTPProbeFenceV1(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if destination == otherDestination || fence == otherFence {
		t.Fatal("URL tamper retained a host destination or fence")
	}
}

func TestHTTPProbeIntentRejectsIdentityAndGuestAuthorityTamper(t *testing.T) {
	command := validHTTPProbeCommandV1()
	for name, mutate := range map[string]func(map[string]any){
		"headers": func(payload map[string]any) {
			payload["headers"] = map[string]string{"Authorization": "guest-owned"}
		},
		"destination": func(payload map[string]any) {
			payload["destination"] = "guest-selected"
		},
	} {
		t.Run(name, func(t *testing.T) {
			payload := map[string]any{
				"version": command.Version, "effect_id": command.EffectID,
				"idempotency_key": command.IdempotencyKey, "kind": command.Kind,
				"request_id": command.RequestID, "component_slug": command.ComponentSlug,
				"url": command.URL, "observed_at": command.ObservedAt,
			}
			mutate(payload)
			raw, err := msgpack.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			intent := Intent{
				Version: VersionV1, ID: command.EffectID, Kind: KindHTTPProbeExecute,
				IdempotencyKey: command.IdempotencyKey, Payload: raw,
			}
			if err := intent.Validate(); err == nil {
				t.Fatal("guest authority widening was accepted")
			}
		})
	}

	command.EffectID = "different"
	raw, err := msgpack.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	intent := Intent{
		Version: VersionV1, ID: "probe-1", Kind: KindHTTPProbeExecute,
		IdempotencyKey: "probe-1", Payload: raw,
	}
	if err := intent.Validate(); err == nil {
		t.Fatal("payload identity mismatch was accepted")
	}
}

func TestHTTPProbeCompletedReceiptRejectsDestinationFenceAndOwnerTamper(t *testing.T) {
	command := validHTTPProbeCommandV1()
	intent, err := NewIntent(
		command.EffectID,
		KindHTTPProbeExecute,
		command.IdempotencyKey,
		command,
	)
	if err != nil {
		t.Fatal(err)
	}
	destination, _ := HTTPProbeDestinationV1(command)
	fence, _ := HTTPProbeFenceV1(command)
	result := HTTPProbeResultV1{
		Contract: HTTPProbeResultContractV1,
		EffectID: command.EffectID, IdempotencyKey: command.IdempotencyKey,
		RequestID: command.RequestID, ComponentSlug: command.ComponentSlug,
		Destination: destination, Fence: fence,
		Status: "operational", Message: "HTTP 204", Transport: "observed",
		HTTPStatus: 204,
		BodySHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ObservedAt: command.ObservedAt,
	}
	receipt, err := NewCompletedReceipt(intent, result)
	if err != nil {
		t.Fatal(err)
	}
	if err := receipt.ValidateFor(intent); err != nil {
		t.Fatal(err)
	}

	for name, mutate := range map[string]func(*HTTPProbeResultV1){
		"destination": func(value *HTTPProbeResultV1) { value.Destination += ".other" },
		"fence":       func(value *HTTPProbeResultV1) { value.Fence += ".other" },
		"owner":       func(value *HTTPProbeResultV1) { value.RequestID = "other-request" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := result
			mutate(&changed)
			raw, err := msgpack.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			tampered := receipt
			tampered.Result = raw
			if err := tampered.ValidateFor(intent); err == nil {
				t.Fatal("tampered HTTP probe receipt was accepted")
			}
		})
	}
}

func validHTTPProbeCommandV1() HTTPProbeCommandV1 {
	return HTTPProbeCommandV1{
		Version:  "sessions.control/v1",
		EffectID: "probe-1", IdempotencyKey: "probe-1",
		Kind: KindHTTPProbeExecute, RequestID: "sweep-1",
		ComponentSlug: "website", URL: "https://sessions.gg/",
		ObservedAt: "2026-07-26T12:00:00Z",
	}
}
