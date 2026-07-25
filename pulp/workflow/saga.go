package workflow

import (
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

// SagaVersionV1 identifies the synchronous, result-capable workflow wire.
// New incompatible fields or semantics require a new version rather than a
// best-effort interpretation by a running application shell.
const SagaVersionV1 = "pulp.workflow.saga.v1"

// FnExecuteSaga is the versioned sibling function exposed by a Lua
// orchestrator for a result-capable saga. A saga persists its state and effect
// intents before a host extension performs any privileged work.
const FnExecuteSaga = "orchestrator.saga.execute.v1"

// SagaStatus is the durable state of a single logical workflow request.
type SagaStatus string

const (
	SagaPending   SagaStatus = "pending"
	SagaCompleted SagaStatus = "completed"
	SagaFailed    SagaStatus = "failed"
)

// EffectAcknowledgementStatus is the durable host acknowledgement state for
// one effect intent. Pending means the intent is committed but not yet
// completed by a host extension; completed and failed are terminal for the
// attempted effect and are safe to replay by idempotency key.
type EffectAcknowledgementStatus string

const (
	EffectPending   EffectAcknowledgementStatus = "pending"
	EffectCompleted EffectAcknowledgementStatus = "completed"
	EffectFailed    EffectAcknowledgementStatus = "failed"
)

// SagaRequest is a versioned, idempotent request to a named Lua-orchestrated
// saga. RequestID correlates the caller with the durable result. IdempotencyKey
// identifies the logical operation and must remain unchanged across retries.
// Payload is a MessagePack object owned by the named saga; use NewSagaRequest
// and DecodePayload to retain a typed boundary at callers.
type SagaRequest struct {
	Version        string             `msgpack:"version"`
	Name           string             `msgpack:"name"`
	RequestID      string             `msgpack:"request_id"`
	IdempotencyKey string             `msgpack:"idempotency_key"`
	Payload        msgpack.RawMessage `msgpack:"payload"`
}

// SagaError is an in-band business or host-operation failure. It keeps a
// failed saga replayable and inspectable without treating an expected failure
// as a sibling-call transport error.
type SagaError struct {
	Code    string `msgpack:"code"`
	Message string `msgpack:"message"`
}

// EffectAcknowledgement records the durable state of a host effect. Result is
// set only once the host has completed the effect. For example, a Stripe
// payment-intent creation acknowledgement can contain the client secret after
// the checkout state and the intent were already committed.
type EffectAcknowledgement struct {
	Status EffectAcknowledgementStatus `msgpack:"status"`
	Result msgpack.RawMessage          `msgpack:"result,omitempty"`
	Error  *SagaError                  `msgpack:"error,omitempty"`
}

// EffectIntent is a durable, host-only operation. It does not execute an
// effect. ID and IdempotencyKey must be stable, allowing a host extension to
// retry a request without issuing duplicate provider calls.
type EffectIntent struct {
	ID              string                `msgpack:"id"`
	Kind            string                `msgpack:"kind"`
	IdempotencyKey  string                `msgpack:"idempotency_key"`
	Payload         msgpack.RawMessage    `msgpack:"payload"`
	Acknowledgement EffectAcknowledgement `msgpack:"acknowledgement"`
}

// SagaResult is the durable outcome returned by FnExecuteSaga. A completed
// result contains a typed MessagePack payload. A pending result exposes only
// committed effect intents, never an invented provider result. A failed result
// carries an in-band SagaError.
type SagaResult struct {
	Version        string             `msgpack:"version"`
	Name           string             `msgpack:"name"`
	RequestID      string             `msgpack:"request_id"`
	IdempotencyKey string             `msgpack:"idempotency_key"`
	Status         SagaStatus         `msgpack:"status"`
	Result         msgpack.RawMessage `msgpack:"result,omitempty"`
	Error          *SagaError         `msgpack:"error,omitempty"`
	Effects        []EffectIntent     `msgpack:"effects,omitempty"`
}

// NewSagaRequest MessagePack-encodes a caller-owned payload into the stable
// saga envelope.
func NewSagaRequest[T any](name, requestID, idempotencyKey string, payload T) (SagaRequest, error) {
	encoded, err := msgpack.Marshal(payload)
	if err != nil {
		return SagaRequest{}, fmt.Errorf("encode saga payload: %w", err)
	}
	request := SagaRequest{
		Version: SagaVersionV1, Name: name, RequestID: requestID,
		IdempotencyKey: idempotencyKey, Payload: encoded,
	}
	if err := request.Validate(); err != nil {
		return SagaRequest{}, err
	}
	return request, nil
}

// DecodePayload decodes a saga-owned payload into a caller-owned type.
func DecodePayload[T any](r SagaRequest) (T, error) {
	var value T
	if len(r.Payload) == 0 {
		return value, fmt.Errorf("saga request has no payload")
	}
	if err := msgpack.Unmarshal(r.Payload, &value); err != nil {
		return value, fmt.Errorf("decode saga payload: %w", err)
	}
	return value, nil
}

// NewCompletedSagaResult builds a completed result after all synchronous host
// effect acknowledgements required by the saga have been durably recorded.
func NewCompletedSagaResult[T any](request SagaRequest, value T, effects []EffectIntent) (SagaResult, error) {
	encoded, err := msgpack.Marshal(value)
	if err != nil {
		return SagaResult{}, fmt.Errorf("encode saga result: %w", err)
	}
	result := SagaResult{
		Version: request.Version, Name: request.Name, RequestID: request.RequestID,
		IdempotencyKey: request.IdempotencyKey, Status: SagaCompleted,
		Result: encoded, Effects: effects,
	}
	if err := result.Validate(); err != nil {
		return SagaResult{}, err
	}
	return result, nil
}

// NewPendingSagaResult exposes a committed saga whose host effects have not
// yet all completed. It intentionally has no result payload.
func NewPendingSagaResult(request SagaRequest, effects []EffectIntent) (SagaResult, error) {
	result := SagaResult{
		Version: request.Version, Name: request.Name, RequestID: request.RequestID,
		IdempotencyKey: request.IdempotencyKey, Status: SagaPending, Effects: effects,
	}
	if err := result.Validate(); err != nil {
		return SagaResult{}, err
	}
	return result, nil
}

// NewFailedSagaResult records an in-band terminal failure for a request.
func NewFailedSagaResult(request SagaRequest, failure SagaError, effects []EffectIntent) (SagaResult, error) {
	result := SagaResult{
		Version: request.Version, Name: request.Name, RequestID: request.RequestID,
		IdempotencyKey: request.IdempotencyKey, Status: SagaFailed,
		Error: &failure, Effects: effects,
	}
	if err := result.Validate(); err != nil {
		return SagaResult{}, err
	}
	return result, nil
}

// DecodeResult decodes a completed saga result into a caller-owned type.
func DecodeResult[T any](result SagaResult) (T, error) {
	var value T
	if result.Status != SagaCompleted {
		return value, fmt.Errorf("saga result is %q, not completed", result.Status)
	}
	if len(result.Result) == 0 {
		return value, fmt.Errorf("completed saga result has no payload")
	}
	if err := msgpack.Unmarshal(result.Result, &value); err != nil {
		return value, fmt.Errorf("decode saga result: %w", err)
	}
	return value, nil
}

// Validate rejects malformed requests before a sibling call is attempted.
func (r SagaRequest) Validate() error {
	if r.Version != SagaVersionV1 {
		return fmt.Errorf("unsupported saga version %q", r.Version)
	}
	if err := validateName("saga name", r.Name); err != nil {
		return err
	}
	if err := validateName("saga request_id", r.RequestID); err != nil {
		return err
	}
	if err := validateName("saga idempotency_key", r.IdempotencyKey); err != nil {
		return err
	}
	if len(r.Payload) == 0 {
		return fmt.Errorf("saga payload is required")
	}
	var ignored any
	if err := msgpack.Unmarshal(r.Payload, &ignored); err != nil {
		return fmt.Errorf("decode saga payload: %w", err)
	}
	return nil
}

// Validate rejects malformed outcomes before a caller trusts a host effect or
// exposes an application result to an HTTP response.
func (r SagaResult) Validate() error {
	if r.Version != SagaVersionV1 {
		return fmt.Errorf("unsupported saga version %q", r.Version)
	}
	if err := validateName("saga name", r.Name); err != nil {
		return err
	}
	if err := validateName("saga request_id", r.RequestID); err != nil {
		return err
	}
	if err := validateName("saga idempotency_key", r.IdempotencyKey); err != nil {
		return err
	}
	seenEffects := make(map[string]struct{}, len(r.Effects))
	seenEffectKeys := make(map[string]struct{}, len(r.Effects))
	for i, effect := range r.Effects {
		if err := effect.Validate(); err != nil {
			return fmt.Errorf("effect %d: %w", i, err)
		}
		if _, exists := seenEffects[effect.ID]; exists {
			return fmt.Errorf("effect %d id %q is duplicated", i, effect.ID)
		}
		seenEffects[effect.ID] = struct{}{}
		if _, exists := seenEffectKeys[effect.IdempotencyKey]; exists {
			return fmt.Errorf("effect %d idempotency_key %q is duplicated", i, effect.IdempotencyKey)
		}
		seenEffectKeys[effect.IdempotencyKey] = struct{}{}
	}
	switch r.Status {
	case SagaPending:
		if len(r.Result) != 0 || r.Error != nil {
			return fmt.Errorf("pending saga must not contain a result or error")
		}
	case SagaCompleted:
		if len(r.Result) == 0 {
			return fmt.Errorf("completed saga result is required")
		}
		var ignored any
		if err := msgpack.Unmarshal(r.Result, &ignored); err != nil {
			return fmt.Errorf("decode completed saga result: %w", err)
		}
		if r.Error != nil {
			return fmt.Errorf("completed saga must not contain an error")
		}
		for i, effect := range r.Effects {
			if effect.Acknowledgement.Status != EffectCompleted {
				return fmt.Errorf("completed saga effect %d is %q", i, effect.Acknowledgement.Status)
			}
		}
	case SagaFailed:
		if len(r.Result) != 0 {
			return fmt.Errorf("failed saga must not contain a result")
		}
		if err := r.Error.Validate(); err != nil {
			return fmt.Errorf("failed saga error: %w", err)
		}
	default:
		return fmt.Errorf("invalid saga status %q", r.Status)
	}
	return nil
}

// Validate rejects malformed durable effect intent acknowledgements.
func (e EffectIntent) Validate() error {
	if err := validateName("effect id", e.ID); err != nil {
		return err
	}
	if err := validateName("effect kind", e.Kind); err != nil {
		return err
	}
	if err := validateName("effect idempotency_key", e.IdempotencyKey); err != nil {
		return err
	}
	if len(e.Payload) == 0 {
		return fmt.Errorf("effect payload is required")
	}
	var ignored any
	if err := msgpack.Unmarshal(e.Payload, &ignored); err != nil {
		return fmt.Errorf("decode effect payload: %w", err)
	}
	return e.Acknowledgement.Validate()
}

// Validate checks acknowledgement/result consistency.
func (a EffectAcknowledgement) Validate() error {
	switch a.Status {
	case EffectPending:
		if len(a.Result) != 0 || a.Error != nil {
			return fmt.Errorf("pending effect acknowledgement must not contain a result or error")
		}
	case EffectCompleted:
		if a.Error != nil {
			return fmt.Errorf("completed effect acknowledgement must not contain an error")
		}
	case EffectFailed:
		if len(a.Result) != 0 {
			return fmt.Errorf("failed effect acknowledgement must not contain a result")
		}
		if err := a.Error.Validate(); err != nil {
			return fmt.Errorf("failed effect acknowledgement error: %w", err)
		}
	default:
		return fmt.Errorf("invalid effect acknowledgement status %q", a.Status)
	}
	return nil
}

// Validate rejects an empty in-band saga error.
func (e *SagaError) Validate() error {
	if e == nil {
		return fmt.Errorf("error is required")
	}
	if err := validateName("saga error code", e.Code); err != nil {
		return err
	}
	if e.Message == "" {
		return fmt.Errorf("saga error message is required")
	}
	return nil
}
