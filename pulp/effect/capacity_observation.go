package effect

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/vmihailenco/msgpack/v5"
)

const (
	CapacityObservationContractV1 = "capacity-observation.v1"
	CapacityObservationFactsMaxV1 = 128

	capacityObservationContractsVersionV1 = "contracts.v1"
	capacityObservationMaxGenerationV1    = uint64(1<<53 - 1)
	capacityObservationMaxCPUMillicoresV1 = int64(1_000_000_000)
)

// CapacityObservationOpaqueRefV1 is a provider-neutral identity. A destination
// is only a host registry key; it is never interpreted as an endpoint.
type CapacityObservationOpaqueRefV1 struct {
	Version string `msgpack:"version"`
	Value   string `msgpack:"value"`
}

// CapacityObservationNodeRefV1 matches contracts.v1 NodeRef without importing
// an application owner package into Fiber.
type CapacityObservationNodeRefV1 struct {
	Version string                         `msgpack:"version"`
	ID      CapacityObservationOpaqueRefV1 `msgpack:"id"`
}

// CapacityObservationFenceV1 binds the read to the exact observation-owner
// lease. The host rejects an expired lease and persists this payload as part of
// the canonical effect intent, so a replay cannot substitute another fence.
type CapacityObservationFenceV1 struct {
	LeaseID            string `msgpack:"lease_id"`
	Value              uint64 `msgpack:"value"`
	ExpiresAtUnixMilli int64  `msgpack:"expires_at_unix_milli"`
}

// CapacityObservationCommandV1 is the complete guest-controlled surface.
// Endpoint, credentials, headers, node ID, HTTP method and response mapping are
// deliberately absent.
type CapacityObservationCommandV1 struct {
	Contract       string                         `msgpack:"contract"`
	Destination    CapacityObservationOpaqueRefV1 `msgpack:"destination"`
	CommandID      string                         `msgpack:"command_id"`
	IdempotencyKey string                         `msgpack:"idempotency_key"`
	Fence          CapacityObservationFenceV1     `msgpack:"fence"`
}

type CapacityObservationResourceQuantityV1 struct {
	Version       string `msgpack:"version"`
	CPUMillicores int64  `msgpack:"cpu_millicores"`
	MemoryBytes   int64  `msgpack:"memory_bytes"`
	StorageBytes  int64  `msgpack:"storage_bytes"`
}

// CapacityObservationFactV1 matches the generic contracts.v1 InventoryFact
// wire shape consumed by workload-inventory. It carries facts, not provider
// response fields or mutation authority.
type CapacityObservationFactV1 struct {
	Version    string                                `msgpack:"version"`
	Node       CapacityObservationNodeRefV1          `msgpack:"node"`
	Generation uint64                                `msgpack:"generation"`
	ObservedAt string                                `msgpack:"observed_at"`
	Capacity   CapacityObservationResourceQuantityV1 `msgpack:"capacity"`
	Allocated  CapacityObservationResourceQuantityV1 `msgpack:"allocated"`
	Reserved   CapacityObservationResourceQuantityV1 `msgpack:"reserved"`
	Available  CapacityObservationResourceQuantityV1 `msgpack:"available"`
}

// CapacityObservationResultV1 is stored inside the canonical pulp.effect.v1
// completed receipt. The outer receipt binds destination and fence; this
// application-facing value exposes only identity and provider-neutral facts.
type CapacityObservationResultV1 struct {
	Contract       string                      `msgpack:"contract"`
	CommandID      string                      `msgpack:"command_id"`
	IdempotencyKey string                      `msgpack:"idempotency_key"`
	Facts          []CapacityObservationFactV1 `msgpack:"facts"`
}

func (value CapacityObservationOpaqueRefV1) Validate() error {
	if value.Version != capacityObservationContractsVersionV1 ||
		capacityObservationTokenV1(value.Value, 256) != nil {
		return fmt.Errorf("capacity observation opaque reference is invalid")
	}
	return nil
}

func (value CapacityObservationNodeRefV1) Validate() error {
	if value.Version != capacityObservationContractsVersionV1 {
		return fmt.Errorf("capacity observation node reference version is invalid")
	}
	return value.ID.Validate()
}

func (value CapacityObservationFenceV1) Validate() error {
	if capacityObservationTokenV1(value.LeaseID, 256) != nil ||
		value.Value == 0 || value.Value > capacityObservationMaxGenerationV1 ||
		value.ExpiresAtUnixMilli <= 0 ||
		value.ExpiresAtUnixMilli > int64(capacityObservationMaxGenerationV1) {
		return fmt.Errorf("capacity observation fence is invalid")
	}
	return nil
}

func (value CapacityObservationCommandV1) Validate() error {
	if value.Contract != CapacityObservationContractV1 ||
		capacityObservationTokenV1(value.CommandID, 256) != nil ||
		capacityObservationTokenV1(value.IdempotencyKey, 256) != nil {
		return fmt.Errorf("capacity observation command identity is invalid")
	}
	if err := value.Destination.Validate(); err != nil {
		return err
	}
	return value.Fence.Validate()
}

func (value CapacityObservationResourceQuantityV1) Validate() error {
	if value.Version != capacityObservationContractsVersionV1 ||
		value.CPUMillicores < 0 || value.CPUMillicores > capacityObservationMaxCPUMillicoresV1 ||
		value.MemoryBytes < 0 || value.StorageBytes < 0 {
		return fmt.Errorf("capacity observation resource quantity is invalid")
	}
	return nil
}

func (value CapacityObservationFactV1) Validate() error {
	if value.Version != capacityObservationContractsVersionV1 ||
		value.Generation == 0 || value.Generation > capacityObservationMaxGenerationV1 {
		return fmt.Errorf("capacity observation fact identity is invalid")
	}
	if err := value.Node.Validate(); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339Nano, value.ObservedAt); err != nil {
		return fmt.Errorf("capacity observation observed_at must be RFC3339")
	}
	for name, quantity := range map[string]CapacityObservationResourceQuantityV1{
		"capacity": value.Capacity, "allocated": value.Allocated,
		"reserved": value.Reserved, "available": value.Available,
	} {
		if err := quantity.Validate(); err != nil {
			return fmt.Errorf("capacity observation %s: %w", name, err)
		}
	}
	if err := capacityObservationQuantityBalanceV1(
		value.Capacity.CPUMillicores, value.Allocated.CPUMillicores,
		value.Reserved.CPUMillicores, value.Available.CPUMillicores,
	); err != nil {
		return fmt.Errorf("capacity observation CPU balance: %w", err)
	}
	if err := capacityObservationQuantityBalanceV1(
		value.Capacity.MemoryBytes, value.Allocated.MemoryBytes,
		value.Reserved.MemoryBytes, value.Available.MemoryBytes,
	); err != nil {
		return fmt.Errorf("capacity observation memory balance: %w", err)
	}
	if err := capacityObservationQuantityBalanceV1(
		value.Capacity.StorageBytes, value.Allocated.StorageBytes,
		value.Reserved.StorageBytes, value.Available.StorageBytes,
	); err != nil {
		return fmt.Errorf("capacity observation storage balance: %w", err)
	}
	return nil
}

func (value CapacityObservationResultV1) Validate() error {
	if value.Contract != CapacityObservationContractV1 ||
		capacityObservationTokenV1(value.CommandID, 256) != nil ||
		capacityObservationTokenV1(value.IdempotencyKey, 256) != nil ||
		len(value.Facts) == 0 || len(value.Facts) > CapacityObservationFactsMaxV1 {
		return fmt.Errorf("capacity observation result identity or fact count is invalid")
	}
	seen := make(map[string]struct{}, len(value.Facts))
	for index, fact := range value.Facts {
		if err := fact.Validate(); err != nil {
			return fmt.Errorf("capacity observation fact %d: %w", index, err)
		}
		key := fact.Node.ID.Value
		if _, exists := seen[key]; exists {
			return fmt.Errorf("capacity observation repeats node %q", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (value CapacityObservationResultV1) ValidateFor(command CapacityObservationCommandV1) error {
	if err := command.Validate(); err != nil {
		return err
	}
	if err := value.Validate(); err != nil {
		return err
	}
	if value.Contract != command.Contract ||
		value.CommandID != command.CommandID ||
		value.IdempotencyKey != command.IdempotencyKey {
		return fmt.Errorf("capacity observation result does not match command")
	}
	return nil
}

func decodeCapacityObservationCommandV1(raw msgpack.RawMessage) (CapacityObservationCommandV1, error) {
	var value CapacityObservationCommandV1
	if err := decodeCapacityObservationStrictV1(raw, &value); err != nil {
		return value, fmt.Errorf("decode capacity observation command: %w", err)
	}
	return value, nil
}

func decodeCapacityObservationResultV1(raw msgpack.RawMessage) (CapacityObservationResultV1, error) {
	var value CapacityObservationResultV1
	if err := decodeCapacityObservationStrictV1(raw, &value); err != nil {
		return value, fmt.Errorf("decode capacity observation result: %w", err)
	}
	return value, nil
}

func decodeCapacityObservationStrictV1(raw []byte, target any) error {
	if len(raw) == 0 {
		return fmt.Errorf("empty MessagePack value")
	}
	reader := bytes.NewReader(raw)
	decoder := msgpack.NewDecoder(reader)
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if reader.Len() != 0 {
		return fmt.Errorf("trailing MessagePack data")
	}
	return nil
}

func capacityObservationQuantityBalanceV1(total, allocated, reserved, available int64) error {
	if allocated > math.MaxInt64-reserved ||
		allocated+reserved > math.MaxInt64-available ||
		allocated+reserved+available != total {
		return fmt.Errorf("allocated + reserved + available must equal capacity")
	}
	return nil
}

func capacityObservationTokenV1(value string, limit int) error {
	if value == "" || len(value) > limit || strings.TrimSpace(value) != value ||
		strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("bounded token is required")
	}
	return nil
}
