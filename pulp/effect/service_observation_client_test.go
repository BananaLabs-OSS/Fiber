package effect

import (
	"errors"
	"testing"

	"github.com/BananaLabs-OSS/Fiber/pulp"
)

func TestExecuteServiceObservationReturnsExactTypedReceipt(t *testing.T) {
	command := serviceObservationCommandForTest()
	intent, err := NewIntent(command.CommandID, KindServiceObservationExecute, command.IdempotencyKey, command)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := NewCompletedReceipt(intent, ServiceObservationResultV1{
		Contract: command.Contract, ServiceDefinitionID: command.ServiceDefinitionID,
		CommandID: command.CommandID, IdempotencyKey: command.IdempotencyKey, Fence: command.Fence,
		Observation: ServiceObservationValueV1{
			Status: ServiceObservationOperationalV1, Evidence: ServiceObservationEvidenceAuthenticatedV1,
			HTTPStatus: 200, ObservedAt: command.ObservedAt,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := MarshalReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	restore := replaceServiceObservationWire(func(request []byte) ([]byte, uint32) {
		got, err := UnmarshalIntent(request)
		if err != nil || got.ID != intent.ID {
			t.Errorf("host request = %#v, %v", got, err)
			return nil, 4
		}
		return wire, 0
	})
	defer restore()
	got, err := ExecuteServiceObservation(intent)
	if err != nil {
		t.Fatal(err)
	}
	if err := got.ValidateFor(intent); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteServiceObservationFailsClosedAndRejectsOtherKinds(t *testing.T) {
	command := serviceObservationCommandForTest()
	intent, err := NewIntent(command.CommandID, KindServiceObservationExecute, command.IdempotencyKey, command)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExecuteServiceObservation(intent); !errors.Is(err, pulp.ErrCapabilityUnavailable) {
		t.Fatalf("native execution error = %v", err)
	}
	other, err := NewIntent(
		"fleet-deprovision", KindFleetServerDeprovision, "fleet-deprovision",
		map[string]string{"server_id": "server-1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	restore := replaceServiceObservationWire(func([]byte) ([]byte, uint32) {
		called = true
		return nil, 0
	})
	defer restore()
	if _, err := ExecuteServiceObservation(other); err == nil || called {
		t.Fatal("non-observation kind reached host")
	}
}

func TestExecuteServiceObservationRejectsMismatchedReceipt(t *testing.T) {
	command := serviceObservationCommandForTest()
	intent, err := NewIntent(command.CommandID, KindServiceObservationExecute, command.IdempotencyKey, command)
	if err != nil {
		t.Fatal(err)
	}
	result := ServiceObservationResultV1{
		Contract: command.Contract, ServiceDefinitionID: command.ServiceDefinitionID,
		CommandID: command.CommandID, IdempotencyKey: command.IdempotencyKey, Fence: command.Fence,
		Observation: ServiceObservationValueV1{
			Status: ServiceObservationOperationalV1, Evidence: ServiceObservationEvidenceAuthenticatedV1,
			ObservedAt: command.ObservedAt,
		},
	}
	receipt, err := NewCompletedReceipt(intent, result)
	if err != nil {
		t.Fatal(err)
	}
	receipt.IntentID = "different"
	wire, err := MarshalReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	restore := replaceServiceObservationWire(func([]byte) ([]byte, uint32) { return wire, 0 })
	defer restore()
	if _, err := ExecuteServiceObservation(intent); err == nil {
		t.Fatal("mismatched receipt accepted")
	}
}

func replaceServiceObservationWire(wire func([]byte) ([]byte, uint32)) func() {
	previous := serviceObservationExecuteWire
	serviceObservationExecuteWire = wire
	return func() { serviceObservationExecuteWire = previous }
}
