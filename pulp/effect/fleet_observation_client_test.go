package effect

import (
	"errors"
	"strings"
	"testing"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

func TestExecuteFleetObservationCanonicalReceipt(t *testing.T) {
	intent := fleetObservationClientIntent(t)
	result := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldStatusV1)
	result.Data.Status = "healthy"
	receipt, err := NewCompletedReceipt(intent, result)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := MarshalReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	restore := replaceFleetObservationWire(t, func(request []byte) ([]byte, uint32) {
		decoded, err := UnmarshalIntent(request)
		if err != nil || decoded.ID != intent.ID || decoded.Kind != intent.Kind ||
			decoded.IdempotencyKey != intent.IdempotencyKey {
			t.Errorf("host intent = %#v, %v", decoded, err)
			return nil, 4
		}
		return wire, 0
	})
	defer restore()

	got, err := ExecuteFleetObservation(intent)
	if err != nil {
		t.Fatalf("ExecuteFleetObservation: %v", err)
	}
	if err := got.ValidateFor(intent); err != nil {
		t.Fatalf("receipt binding: %v", err)
	}
}

func TestExecuteFleetObservationFailsClosedAndRejectsOtherFleetKinds(t *testing.T) {
	intent := fleetObservationClientIntent(t)
	if _, err := ExecuteFleetObservation(intent); !errors.Is(err, pulp.ErrCapabilityUnavailable) {
		t.Fatalf("native ExecuteFleetObservation = %v, want ErrCapabilityUnavailable", err)
	}

	other, err := NewIntent("fleet-deprovision", KindFleetServerDeprovision, "fleet-deprovision", map[string]string{"server_id": "server-1"})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	restore := replaceFleetObservationWire(t, func(request []byte) ([]byte, uint32) {
		called = true
		return nil, 0
	})
	defer restore()
	if _, err := ExecuteFleetObservation(other); err == nil {
		t.Fatal("non-observation Fleet kind executed")
	}
	if called {
		t.Fatal("host called for non-observation Fleet kind")
	}
}

func TestExecuteFleetObservationRejectsMismatchedOrExtendedReceipt(t *testing.T) {
	intent := fleetObservationClientIntent(t)
	result := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldStatusV1)
	result.Data.Status = "healthy"
	receipt, err := NewCompletedReceipt(intent, result)
	if err != nil {
		t.Fatal(err)
	}
	receipt.IntentID = "other"
	wire, err := MarshalReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	restore := replaceFleetObservationWire(t, func(request []byte) ([]byte, uint32) {
		return wire, 0
	})
	if _, err := ExecuteFleetObservation(intent); err == nil {
		t.Fatal("mismatched host receipt validated")
	}
	restore()

	valid, err := NewCompletedReceipt(intent, result)
	if err != nil {
		t.Fatal(err)
	}
	validWire, err := MarshalReceipt(valid)
	if err != nil {
		t.Fatal(err)
	}
	// Append an unknown envelope field by decoding into a map and remarshal.
	var envelope map[string]any
	if err := msgpack.Unmarshal(validWire, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope["endpoint"] = "https://not-allowed.invalid"
	extended, err := msgpack.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	restore = replaceFleetObservationWire(t, func(request []byte) ([]byte, uint32) {
		return extended, 0
	})
	defer restore()
	if _, err := ExecuteFleetObservation(intent); err == nil {
		t.Fatal("extended host receipt envelope validated")
	}
}

func TestFleetObservationCodeError(t *testing.T) {
	if err := fleetObservationCodeError(0); err != nil {
		t.Fatalf("code 0: %v", err)
	}
	for _, code := range []uint32{1, 2, 3, 4, 5, 6, 77} {
		if err := fleetObservationCodeError(code); err == nil {
			t.Errorf("code %d did not fail", code)
		}
	}
	for _, code := range []uint32{10, 99} {
		if err := fleetObservationCodeError(code); !errors.Is(err, pulp.ErrCapabilityUnavailable) {
			t.Fatalf("code %d = %v, want ErrCapabilityUnavailable", code, err)
		}
	}
}

func fleetObservationClientIntent(t *testing.T) Intent {
	t.Helper()
	revision := "fleet-live-v1:" + strings.Repeat("a", 64)
	intent, err := NewIntent("fleet-observation-1", KindFleetRuntimeObservationExecute, "fleet-observation:server-1:status", FleetRuntimeObservationIntentV1{
		Contract: FleetRuntimeObservationContractV1, ServerID: "server-1", NodeID: "node-1",
		ContainerID: "container-1", Field: FleetRuntimeObservationFieldStatusV1,
		Generation: revision, SourceRevision: revision,
	})
	if err != nil {
		t.Fatalf("NewIntent: %v", err)
	}
	return intent
}

func replaceFleetObservationWire(t *testing.T, wire func([]byte) ([]byte, uint32)) func() {
	t.Helper()
	previous := fleetObservationExecuteWire
	fleetObservationExecuteWire = wire
	return func() { fleetObservationExecuteWire = previous }
}
