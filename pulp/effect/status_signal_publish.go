package effect

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

// StatusSignalTarget is a fixed component identifier understood by the
// status ingest contract. It is deliberately not a free-form routing field.
type StatusSignalTarget string

const (
	StatusSignalTargetPayments    StatusSignalTarget = "payments"
	StatusSignalTargetProvisioner StatusSignalTarget = "provisioner"
	StatusSignalTargetEmail       StatusSignalTarget = "email"
	StatusSignalTargetResolver    StatusSignalTarget = "resolver"
)

// StatusSignalState is the bounded health state emitted for a component.
type StatusSignalState string

const (
	StatusSignalOK       StatusSignalState = "ok"
	StatusSignalDegraded StatusSignalState = "degraded"
	StatusSignalDown     StatusSignalState = "down"
)

const maxStatusSignalDetailBytes = 512

// StatusSignalPublishPayload is the complete guest-owned portion of a status
// publish request. Endpoint, bearer token, HTTP method, and headers remain
// host scope configuration; this payload cannot become a generic HTTP call.
type StatusSignalPublishPayload struct {
	Target        StatusSignalTarget `msgpack:"target"`
	Signal        StatusSignalState  `msgpack:"signal"`
	Detail        string             `msgpack:"detail"`
	ExpiresAtUnix int64              `msgpack:"expires_at_unix"`
}

// StatusSignalPublishResult is the stable acknowledgement stored with a
// completed receipt. It echoes the bounded identity fields, allowing the
// owner to prove a host receipt belongs to the exact planned signal.
type StatusSignalPublishResult struct {
	Target        StatusSignalTarget `msgpack:"target"`
	Signal        StatusSignalState  `msgpack:"signal"`
	ExpiresAtUnix int64              `msgpack:"expires_at_unix"`
}

func (p StatusSignalPublishPayload) Validate() error {
	if !validStatusSignalTarget(p.Target) {
		return fmt.Errorf("status signal target %q is not allowed", p.Target)
	}
	if !validStatusSignalState(p.Signal) {
		return fmt.Errorf("status signal state %q is not allowed", p.Signal)
	}
	if strings.TrimSpace(p.Detail) != p.Detail || p.Detail == "" {
		return fmt.Errorf("status signal detail is required and must not have surrounding whitespace")
	}
	if len(p.Detail) > maxStatusSignalDetailBytes {
		return fmt.Errorf("status signal detail exceeds %d bytes", maxStatusSignalDetailBytes)
	}
	for _, r := range p.Detail {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("status signal detail contains a control character")
		}
	}
	if p.ExpiresAtUnix <= 0 {
		return fmt.Errorf("status signal expires_at_unix must be positive")
	}
	if time.Unix(p.ExpiresAtUnix, 0).UTC().Year() > 9999 {
		return fmt.Errorf("status signal expires_at_unix is out of range")
	}
	return nil
}

func (r StatusSignalPublishResult) Validate() error {
	return StatusSignalPublishPayload{
		Target: r.Target, Signal: r.Signal, Detail: "ack", ExpiresAtUnix: r.ExpiresAtUnix,
	}.Validate()
}

func decodeStatusSignalPublishPayload(raw msgpack.RawMessage) (StatusSignalPublishPayload, error) {
	var payload StatusSignalPublishPayload
	decoder := msgpack.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&payload); err != nil {
		return payload, fmt.Errorf("decode status signal publish payload: %w", err)
	}
	return payload, nil
}

func decodeStatusSignalPublishResult(raw msgpack.RawMessage) (StatusSignalPublishResult, error) {
	var result StatusSignalPublishResult
	decoder := msgpack.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("decode status signal publish result: %w", err)
	}
	return result, nil
}

func validStatusSignalTarget(target StatusSignalTarget) bool {
	switch target {
	case StatusSignalTargetPayments, StatusSignalTargetProvisioner,
		StatusSignalTargetEmail, StatusSignalTargetResolver:
		return true
	default:
		return false
	}
}

func validStatusSignalState(state StatusSignalState) bool {
	switch state {
	case StatusSignalOK, StatusSignalDegraded, StatusSignalDown:
		return true
	default:
		return false
	}
}
