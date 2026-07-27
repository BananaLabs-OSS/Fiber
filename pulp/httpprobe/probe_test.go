package httpprobe

import (
	"errors"
	"testing"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

func TestExecuteValidatesFencedReplayReceipt(t *testing.T) {
	old := executeWire
	t.Cleanup(func() { executeWire = old })
	intent := Intent{Version: VersionV1, IntentID: "probe-1", IdempotencyKey: "probe-1", Fence: "lease-8", Destination: "public-api"}
	receipt := Receipt{Version: VersionV1, IntentID: intent.IntentID, IdempotencyKey: intent.IdempotencyKey, Fence: intent.Fence, Destination: intent.Destination, Transport: "observed", HTTPStatus: 204, BodySHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	wire, err := msgpack.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	executeWire = func([]byte) ([]byte, uint32) { return wire, 0 }
	got, err := Execute(intent)
	if err != nil || got != receipt {
		t.Fatalf("Execute = %#v, %v", got, err)
	}
}

func TestExecuteRejectsMismatchedFenceAndUnknownReceiptFields(t *testing.T) {
	old := executeWire
	t.Cleanup(func() { executeWire = old })
	intent := Intent{Version: VersionV1, IntentID: "probe-1", IdempotencyKey: "probe-1", Fence: "lease-8", Destination: "public-api"}
	bad := Receipt{Version: VersionV1, IntentID: intent.IntentID, IdempotencyKey: intent.IdempotencyKey, Fence: "lease-9", Destination: intent.Destination, Transport: "timeout"}
	wire, err := msgpack.Marshal(bad)
	if err != nil {
		t.Fatal(err)
	}
	executeWire = func([]byte) ([]byte, uint32) { return wire, 0 }
	if _, err := Execute(intent); err == nil {
		t.Fatal("mismatched fence accepted")
	}
	unknown, err := msgpack.Marshal(map[string]any{"version": VersionV1, "intent_id": "probe-1", "idempotency_key": "probe-1", "fence": "lease-8", "destination": "public-api", "transport": "timeout", "url": "https://guest.example"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeReceipt(unknown); err == nil {
		t.Fatal("unknown receipt field accepted")
	}
}

func TestNativeSeamFailsClosed(t *testing.T) {
	old := executeWire
	t.Cleanup(func() { executeWire = old })
	executeWire = func([]byte) ([]byte, uint32) { return nil, 99 }
	_, err := Execute(Intent{Version: VersionV1, IntentID: "probe-1", IdempotencyKey: "probe-1", Fence: "lease-8", Destination: "public-api"})
	if !errors.Is(err, pulp.ErrCapabilityUnavailable) {
		t.Fatalf("error = %v", err)
	}
}
