package effect

import (
	"fmt"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

// PublishStatusSignal sends one canonical status signal intent to the narrow
// effect.status.signal capability. A cell using it must declare that exact
// capability; this is not a general workers or HTTP client.
func PublishStatusSignal(intent Intent) (Receipt, error) {
	if err := intent.Validate(); err != nil {
		return Receipt{}, fmt.Errorf("effect.status.signal: invalid intent: %w", err)
	}
	if intent.Kind != KindStatusSignalPublish {
		return Receipt{}, fmt.Errorf("effect.status.signal: unsupported intent kind %q", intent.Kind)
	}
	request, err := MarshalIntent(intent)
	if err != nil {
		return Receipt{}, fmt.Errorf("effect.status.signal: marshal intent: %w", err)
	}
	wire, code := statusSignalPublishWire(request)
	if err := statusSignalCodeError(code); err != nil {
		return Receipt{}, err
	}
	if len(wire) == 0 {
		return Receipt{}, fmt.Errorf("effect.status.signal: empty receipt response")
	}
	var receipt Receipt
	if err := msgpack.Unmarshal(wire, &receipt); err != nil {
		return Receipt{}, fmt.Errorf("effect.status.signal: decode receipt: %w", err)
	}
	if err := receipt.ValidateFor(intent); err != nil {
		return Receipt{}, fmt.Errorf("effect.status.signal: invalid receipt: %w", err)
	}
	return receipt, nil
}

func statusSignalCodeError(code uint32) error {
	switch code {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("effect.status.signal: empty request")
	case 2:
		return fmt.Errorf("effect.status.signal: request memory read failed")
	case 3:
		return fmt.Errorf("effect.status.signal: request decode failed")
	case 4:
		return fmt.Errorf("effect.status.signal: invalid effect intent")
	case 5:
		return fmt.Errorf("effect.status.signal: status signal execution failed")
	case 6:
		return fmt.Errorf("effect.status.signal: response allocation or write failed")
	case 10:
		return fmt.Errorf("effect.status.signal: host status runtime unavailable")
	case 99:
		return fmt.Errorf("effect.status.signal: %w", pulp.ErrCapabilityUnavailable)
	default:
		return fmt.Errorf("effect.status.signal: host code %d", code)
	}
}
