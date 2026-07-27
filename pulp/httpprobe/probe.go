// Package httpprobe executes one fenced HTTP observation through the narrow
// effect.http.probe.v1 host capability. A guest supplies only a destination
// key; the host owns the URL, method, timeout, and response-body limit.
package httpprobe

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	Capability = "effect.http.probe.v1"
	Import     = "http_probe_execute"
	VersionV1  = "http-probe.v1"

	maxFieldBytes = 256
)

// Intent is a durable, versioned request for one host-owned HTTP observation.
// Destination selects a host-configured allowlist entry. Fence is opaque
// owner state that is echoed in the receipt, binding a completion to exactly
// the attempt that requested it.
type Intent struct {
	Version        string `msgpack:"version"`
	IntentID       string `msgpack:"intent_id"`
	IdempotencyKey string `msgpack:"idempotency_key"`
	Fence          string `msgpack:"fence"`
	Destination    string `msgpack:"destination"`
}

// Receipt is the replay-stable result of an HTTP observation. BodySHA256 and
// BodyBytes describe a bounded body without returning its potentially
// sensitive contents to the guest. HTTPStatus is zero when no response was
// received. Transport is one of observed, timeout, network, redirect,
// body_too_large, or invalid_response.
type Receipt struct {
	Version        string `msgpack:"version"`
	IntentID       string `msgpack:"intent_id"`
	IdempotencyKey string `msgpack:"idempotency_key"`
	Fence          string `msgpack:"fence"`
	Destination    string `msgpack:"destination"`
	Transport      string `msgpack:"transport"`
	HTTPStatus     uint16 `msgpack:"http_status"`
	BodyBytes      uint32 `msgpack:"body_bytes"`
	BodySHA256     string `msgpack:"body_sha256,omitempty"`
}

// Execute sends one canonical intent to the host. Repeating the exact same
// idempotency key and fence returns the host's original encoded receipt.
func Execute(intent Intent) (Receipt, error) {
	if err := intent.Validate(); err != nil {
		return Receipt{}, fmt.Errorf("%s: invalid intent: %w", Capability, err)
	}
	wire, err := msgpack.Marshal(intent)
	if err != nil {
		return Receipt{}, fmt.Errorf("%s: marshal intent: %w", Capability, err)
	}
	response, code := executeWire(wire)
	if err := codeError(code); err != nil {
		return Receipt{}, err
	}
	receipt, err := DecodeReceipt(response)
	if err != nil {
		return Receipt{}, fmt.Errorf("%s: decode receipt: %w", Capability, err)
	}
	if err := receipt.ValidateFor(intent); err != nil {
		return Receipt{}, fmt.Errorf("%s: invalid receipt: %w", Capability, err)
	}
	return receipt, nil
}

func (i Intent) Validate() error {
	if i.Version != VersionV1 {
		return fmt.Errorf("unsupported version %q", i.Version)
	}
	if err := validateField("intent_id", i.IntentID); err != nil {
		return err
	}
	if err := validateField("idempotency_key", i.IdempotencyKey); err != nil {
		return err
	}
	if err := validateField("fence", i.Fence); err != nil {
		return err
	}
	if err := validateDestination(i.Destination); err != nil {
		return err
	}
	return nil
}

func (r Receipt) Validate() error {
	if r.Version != VersionV1 {
		return fmt.Errorf("unsupported version %q", r.Version)
	}
	if err := validateField("receipt intent_id", r.IntentID); err != nil {
		return err
	}
	if err := validateField("receipt idempotency_key", r.IdempotencyKey); err != nil {
		return err
	}
	if err := validateField("receipt fence", r.Fence); err != nil {
		return err
	}
	if err := validateDestination(r.Destination); err != nil {
		return err
	}
	switch r.Transport {
	case "observed":
		if r.HTTPStatus == 0 || r.BodySHA256 == "" || !isSHA256(r.BodySHA256) {
			return errors.New("observed receipt requires an HTTP status and body digest")
		}
	case "timeout", "network", "redirect", "body_too_large", "invalid_response":
		if r.HTTPStatus != 0 || r.BodyBytes != 0 || r.BodySHA256 != "" {
			return fmt.Errorf("%s receipt cannot contain an HTTP response", r.Transport)
		}
	default:
		return fmt.Errorf("unsupported transport result %q", r.Transport)
	}
	return nil
}

// ValidateFor proves the receipt is fenced to this exact intent.
func (r Receipt) ValidateFor(intent Intent) error {
	if err := intent.Validate(); err != nil {
		return err
	}
	if err := r.Validate(); err != nil {
		return err
	}
	if r.IntentID != intent.IntentID || r.IdempotencyKey != intent.IdempotencyKey || r.Fence != intent.Fence || r.Destination != intent.Destination {
		return errors.New("receipt does not match intent")
	}
	return nil
}

// DecodeIntent and DecodeReceipt reject unknown fields and trailing values so
// a future ABI cannot silently widen this v1 capability.
func DecodeIntent(wire []byte) (Intent, error) {
	var intent Intent
	if err := decodeStrict(wire, &intent); err != nil {
		return Intent{}, err
	}
	return intent, intent.Validate()
}

func DecodeReceipt(wire []byte) (Receipt, error) {
	var receipt Receipt
	if err := decodeStrict(wire, &receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, receipt.Validate()
}

func decodeStrict(wire []byte, out any) error {
	if len(wire) == 0 {
		return errors.New("empty MessagePack value")
	}
	decoder := msgpack.NewDecoder(bytes.NewReader(wire))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing MessagePack value")
		}
		return err
	}
	return nil
}

func validateField(label, value string) error {
	if value == "" || strings.TrimSpace(value) != value || len(value) > maxFieldBytes {
		return fmt.Errorf("%s must be a non-empty trimmed value of at most %d bytes", label, maxFieldBytes)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%s contains a control character", label)
		}
	}
	return nil
}

func validateDestination(value string) error {
	if err := validateField("destination", value); err != nil {
		return err
	}
	for _, r := range value {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-') {
			return errors.New("destination contains an unsupported character")
		}
	}
	return nil
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

func codeError(code uint32) error {
	switch code {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("%s: empty request", Capability)
	case 2:
		return fmt.Errorf("%s: request memory read failed", Capability)
	case 3:
		return fmt.Errorf("%s: invalid request", Capability)
	case 4:
		return fmt.Errorf("%s: destination is not configured", Capability)
	case 5:
		return fmt.Errorf("%s: idempotency or fence conflict", Capability)
	case 6:
		return fmt.Errorf("%s: response allocation or write failed", Capability)
	case 99:
		return fmt.Errorf("%s: %w", Capability, pulp.ErrCapabilityUnavailable)
	default:
		return fmt.Errorf("%s: host code %d", Capability, code)
	}
}
