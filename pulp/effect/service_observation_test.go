package effect

import (
	"strings"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestServiceObservationContractBindsOpaqueDefinitionCommandAndFence(t *testing.T) {
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
			HTTPStatus: 200, ObservedAt: command.ObservedAt,
		},
	}
	receipt, err := NewCompletedReceipt(intent, result)
	if err != nil {
		t.Fatal(err)
	}
	if err := receipt.ValidateFor(intent); err != nil {
		t.Fatal(err)
	}
}

func TestServiceObservationContractRejectsTransportAndIdentityTamper(t *testing.T) {
	command := serviceObservationCommandForTest()
	raw, err := msgpack.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	var extended map[string]any
	if err := msgpack.Unmarshal(raw, &extended); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"url", "authorization", "headers", "secret", "provider"} {
		extended[forbidden] = "not allowed"
		wire, err := msgpack.Marshal(extended)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := decodeServiceObservationCommandV1(wire); err == nil {
			t.Fatalf("command accepted %q", forbidden)
		}
		delete(extended, forbidden)
	}

	mismatch := command
	mismatch.IdempotencyKey = "different"
	if _, err := NewIntent(command.CommandID, KindServiceObservationExecute, command.IdempotencyKey, mismatch); err == nil {
		t.Fatal("intent accepted mismatched payload idempotency")
	}
}

func TestServiceObservationContractBoundsReceiptEvidence(t *testing.T) {
	command := serviceObservationCommandForTest()
	base := ServiceObservationResultV1{
		Contract: command.Contract, ServiceDefinitionID: command.ServiceDefinitionID,
		CommandID: command.CommandID, IdempotencyKey: command.IdempotencyKey, Fence: command.Fence,
		Observation: ServiceObservationValueV1{
			Status: ServiceObservationMajorV1, Evidence: ServiceObservationEvidenceUnavailableV1,
			ObservedAt: command.ObservedAt,
		},
	}
	if err := base.ValidateFor(command); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*ServiceObservationResultV1){
		"unknown status":   func(v *ServiceObservationResultV1) { v.Observation.Status = "reachable" },
		"unknown evidence": func(v *ServiceObservationResultV1) { v.Observation.Evidence = "provider_error" },
		"oversize message": func(v *ServiceObservationResultV1) { v.Observation.Message = strings.Repeat("x", 513) },
		"bad HTTP status":  func(v *ServiceObservationResultV1) { v.Observation.HTTPStatus = 99 },
		"changed fence":    func(v *ServiceObservationResultV1) { v.Fence.Attempt++ },
	} {
		t.Run(name, func(t *testing.T) {
			changed := base
			mutate(&changed)
			if err := changed.ValidateFor(command); err == nil {
				t.Fatal("tampered observation validated")
			}
		})
	}
}

func serviceObservationCommandForTest() ServiceObservationCommandV1 {
	return ServiceObservationCommandV1{
		Contract: ServiceObservationContractV1, ServiceDefinitionID: "sessions.stripe.primary",
		CommandID: "observe-stripe-1", IdempotencyKey: "observe-stripe-1",
		Fence: ServiceObservationFenceV1{
			LeaseID: "lease-stripe-1", Attempt: 2, LeaseExpiresAt: "2026-07-26T13:00:00Z",
		},
		ObservedAt: "2026-07-26T12:00:00Z",
	}
}
