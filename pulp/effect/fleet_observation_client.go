package effect

import (
	"bytes"
	"fmt"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

// ExecuteFleetObservation executes one canonical, bounded live-observation
// intent through the effect.fleet.observation capability. It admits no other
// Fleet kind and returns only a receipt bound to the exact intent identity.
func ExecuteFleetObservation(intent Intent) (Receipt, error) {
	if err := intent.Validate(); err != nil {
		return Receipt{}, fmt.Errorf("effect.fleet.observation: invalid intent: %w", err)
	}
	if intent.Kind != KindFleetRuntimeObservationExecute {
		return Receipt{}, fmt.Errorf("effect.fleet.observation: unsupported intent kind %q", intent.Kind)
	}

	request, err := MarshalIntent(intent)
	if err != nil {
		return Receipt{}, fmt.Errorf("effect.fleet.observation: marshal intent: %w", err)
	}
	response, code := fleetObservationExecuteWire(request)
	if err := fleetObservationCodeError(code); err != nil {
		return Receipt{}, err
	}
	if len(response) == 0 {
		return Receipt{}, fmt.Errorf("effect.fleet.observation: empty receipt response")
	}

	// Host receipts must already be canonical. Do not normalize aliases or
	// ignore extra envelope fields at this privileged boundary.
	var receipt Receipt
	decoder := msgpack.NewDecoder(bytes.NewReader(response))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&receipt); err != nil {
		return Receipt{}, fmt.Errorf("effect.fleet.observation: decode receipt: %w", err)
	}
	if receipt.Status != Completed {
		return Receipt{}, fmt.Errorf("effect.fleet.observation: synchronous receipt must be completed")
	}
	if err := receipt.ValidateFor(intent); err != nil {
		return Receipt{}, fmt.Errorf("effect.fleet.observation: invalid receipt: %w", err)
	}
	return receipt, nil
}

func fleetObservationCodeError(code uint32) error {
	switch code {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("effect.fleet.observation: empty or invalid request")
	case 2:
		return fmt.Errorf("effect.fleet.observation: request memory read failed")
	case 3:
		return fmt.Errorf("effect.fleet.observation: request decode failed")
	case 4:
		return fmt.Errorf("effect.fleet.observation: unsupported or invalid observation intent")
	case 5:
		return fmt.Errorf("effect.fleet.observation: observation execution failed")
	case 6:
		return fmt.Errorf("effect.fleet.observation: response allocation or write failed")
	case 10, 99:
		return fmt.Errorf("effect.fleet.observation: %w", pulp.ErrCapabilityUnavailable)
	default:
		return fmt.Errorf("effect.fleet.observation: host code %d", code)
	}
}
