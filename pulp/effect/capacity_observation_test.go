package effect

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

func capacityObservationCommandForTest() CapacityObservationCommandV1 {
	return CapacityObservationCommandV1{
		Contract: CapacityObservationContractV1,
		Destination: CapacityObservationOpaqueRefV1{
			Version: "contracts.v1", Value: "capacity.primary",
		},
		CommandID: "capacity-observe-1", IdempotencyKey: "capacity-observe-1",
		Fence: CapacityObservationFenceV1{
			LeaseID: "capacity-lease-1", Value: 7,
			ExpiresAtUnixMilli: time.Date(2026, 7, 26, 12, 1, 0, 0, time.UTC).UnixMilli(),
		},
	}
}

func capacityObservationFactForTest() CapacityObservationFactV1 {
	quantity := func(cpu, memory, storage int64) CapacityObservationResourceQuantityV1 {
		return CapacityObservationResourceQuantityV1{
			Version: "contracts.v1", CPUMillicores: cpu,
			MemoryBytes: memory, StorageBytes: storage,
		}
	}
	return CapacityObservationFactV1{
		Version: "contracts.v1",
		Node: CapacityObservationNodeRefV1{
			Version: "contracts.v1",
			ID:      CapacityObservationOpaqueRefV1{Version: "contracts.v1", Value: "node-1"},
		},
		Generation: 1785067200000, ObservedAt: "2026-07-26T12:00:00Z",
		Capacity:  quantity(8000, 32<<30, 200<<30),
		Allocated: quantity(2500, 10<<30, 50<<30),
		Reserved:  quantity(0, 0, 0),
		Available: quantity(5500, 22<<30, 150<<30),
	}
}

func TestCapacityObservationContractRoundTripAndExactReceipt(t *testing.T) {
	command := capacityObservationCommandForTest()
	intent, err := NewIntent(
		command.CommandID, KindCapacityObservationExecute, command.IdempotencyKey, command,
	)
	if err != nil {
		t.Fatal(err)
	}
	result := CapacityObservationResultV1{
		Contract: command.Contract, CommandID: command.CommandID,
		IdempotencyKey: command.IdempotencyKey,
		Facts:          []CapacityObservationFactV1{capacityObservationFactForTest()},
	}
	receipt, err := NewCompletedReceipt(intent, result)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := MarshalReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := UnmarshalReceipt(wire)
	if err != nil || !reflect.DeepEqual(decoded, receipt) {
		t.Fatalf("decoded receipt = %#v, %v", decoded, err)
	}
}

func TestCapacityObservationContractRejectsAuthorityAndTamper(t *testing.T) {
	command := capacityObservationCommandForTest()
	raw, err := msgpack.Marshal(map[string]any{
		"contract": command.Contract, "destination": command.Destination,
		"command_id": command.CommandID, "idempotency_key": command.IdempotencyKey,
		"fence": command.Fence, "url": "https://not-allowed.invalid",
	})
	if err != nil {
		t.Fatal(err)
	}
	intent := Intent{
		Version: VersionV1, ID: command.CommandID, Kind: KindCapacityObservationExecute,
		IdempotencyKey: command.IdempotencyKey, Payload: raw,
	}
	if err := intent.Validate(); err == nil {
		t.Fatal("extended capacity command was accepted")
	}

	command.Fence.Value = 0
	if _, err := NewIntent(
		command.CommandID, KindCapacityObservationExecute, command.IdempotencyKey, command,
	); err == nil {
		t.Fatal("zero fence was accepted")
	}
}

func TestCapacityObservationResultRejectsDuplicateOrUnbalancedFacts(t *testing.T) {
	command := capacityObservationCommandForTest()
	fact := capacityObservationFactForTest()
	result := CapacityObservationResultV1{
		Contract: command.Contract, CommandID: command.CommandID,
		IdempotencyKey: command.IdempotencyKey,
		Facts:          []CapacityObservationFactV1{fact, fact},
	}
	if err := result.Validate(); err == nil {
		t.Fatal("duplicate node facts were accepted")
	}
	result.Facts = []CapacityObservationFactV1{fact}
	result.Facts[0].Available.CPUMillicores--
	if err := result.Validate(); err == nil {
		t.Fatal("unbalanced resources were accepted")
	}
}

func TestExecuteCapacityObservationUsesExactCapabilityReceipt(t *testing.T) {
	command := capacityObservationCommandForTest()
	intent, err := NewIntent(
		command.CommandID, KindCapacityObservationExecute, command.IdempotencyKey, command,
	)
	if err != nil {
		t.Fatal(err)
	}
	want, err := NewCompletedReceipt(intent, CapacityObservationResultV1{
		Contract: command.Contract, CommandID: command.CommandID,
		IdempotencyKey: command.IdempotencyKey,
		Facts:          []CapacityObservationFactV1{capacityObservationFactForTest()},
	})
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := MarshalReceipt(want)
	previous := capacityObservationExecuteWire
	t.Cleanup(func() { capacityObservationExecuteWire = previous })
	capacityObservationExecuteWire = func([]byte) ([]byte, uint32) {
		return wire, 0
	}
	got, err := ExecuteCapacityObservation(intent)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("receipt = %#v, %v", got, err)
	}
}

func TestExecuteCapacityObservationFailsClosed(t *testing.T) {
	command := capacityObservationCommandForTest()
	intent, err := NewIntent(
		command.CommandID, KindCapacityObservationExecute, command.IdempotencyKey, command,
	)
	if err != nil {
		t.Fatal(err)
	}
	previous := capacityObservationExecuteWire
	t.Cleanup(func() { capacityObservationExecuteWire = previous })
	capacityObservationExecuteWire = func([]byte) ([]byte, uint32) {
		return nil, 99
	}
	if _, err := ExecuteCapacityObservation(intent); !errors.Is(err, pulp.ErrCapabilityUnavailable) {
		t.Fatalf("error = %v", err)
	}
}
