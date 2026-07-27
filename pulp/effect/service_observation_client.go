package effect

import (
	"bytes"
	"fmt"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

// ExecuteServiceObservation executes one authenticated service observation
// through the effect.service.observation host capability.
func ExecuteServiceObservation(intent Intent) (Receipt, error) {
	if err := intent.Validate(); err != nil {
		return Receipt{}, fmt.Errorf("effect.service.observation: invalid intent: %w", err)
	}
	if intent.Kind != KindServiceObservationExecute {
		return Receipt{}, fmt.Errorf("effect.service.observation: unsupported intent kind %q", intent.Kind)
	}
	request, err := MarshalIntent(intent)
	if err != nil {
		return Receipt{}, fmt.Errorf("effect.service.observation: marshal intent: %w", err)
	}
	response, code := serviceObservationExecuteWire(request)
	if err := serviceObservationCodeError(code); err != nil {
		return Receipt{}, err
	}
	if len(response) == 0 {
		return Receipt{}, fmt.Errorf("effect.service.observation: empty receipt response")
	}
	reader := bytes.NewReader(response)
	decoder := msgpack.NewDecoder(reader)
	decoder.DisallowUnknownFields(true)
	var receipt Receipt
	if err := decoder.Decode(&receipt); err != nil || reader.Len() != 0 {
		return Receipt{}, fmt.Errorf("effect.service.observation: decode receipt failed")
	}
	if receipt.Status != Completed {
		return Receipt{}, fmt.Errorf("effect.service.observation: synchronous receipt must be completed")
	}
	if err := receipt.ValidateFor(intent); err != nil {
		return Receipt{}, fmt.Errorf("effect.service.observation: invalid receipt: %w", err)
	}
	return receipt, nil
}

func serviceObservationCodeError(code uint32) error {
	switch code {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("effect.service.observation: empty or invalid request")
	case 2:
		return fmt.Errorf("effect.service.observation: request memory read failed")
	case 3:
		return fmt.Errorf("effect.service.observation: request decode failed")
	case 4:
		return fmt.Errorf("effect.service.observation: unsupported or invalid observation intent")
	case 5:
		return fmt.Errorf("effect.service.observation: observation execution failed")
	case 6:
		return fmt.Errorf("effect.service.observation: response allocation or write failed")
	case 10, 99:
		return fmt.Errorf("effect.service.observation: %w", pulp.ErrCapabilityUnavailable)
	default:
		return fmt.Errorf("effect.service.observation: host code %d", code)
	}
}
