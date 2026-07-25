package workflow

import (
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

type callFunc func(target, function string, payload []byte) ([]byte, error)

// Client invokes a manifest-authorized orchestration cell.
type Client struct {
	// Name is the target cell identifier. The caller's pulp.cell.toml must
	// consume this cell name or one of its provided capabilities.
	Name string

	call callFunc
}

// NewClient creates a client for a loaded orchestration cell.
func NewClient(name string) *Client {
	return &Client{Name: name, call: defaultPulpCall}
}

// Dispatch validates and sends one event to the orchestration cell, then
// validates the returned action envelopes before exposing them to the caller.
func (c *Client) Dispatch(request DispatchRequest) (DispatchResult, error) {
	if c == nil {
		return DispatchResult{}, fmt.Errorf("workflow client is nil")
	}
	if err := validateName("orchestrator cell name", c.Name); err != nil {
		return DispatchResult{}, err
	}
	if err := request.Validate(); err != nil {
		return DispatchResult{}, fmt.Errorf("validate dispatch request: %w", err)
	}
	encoded, err := msgpack.Marshal(request)
	if err != nil {
		return DispatchResult{}, fmt.Errorf("encode dispatch request: %w", err)
	}
	caller := c.call
	if caller == nil {
		caller = defaultPulpCall
	}
	response, err := caller(c.Name, FnDispatch, encoded)
	if err != nil {
		return DispatchResult{}, fmt.Errorf("dispatch %q: %w", request.Event, err)
	}
	if len(response) == 0 {
		return DispatchResult{}, fmt.Errorf("dispatch %q returned an empty response", request.Event)
	}
	var result DispatchResult
	if err := msgpack.Unmarshal(response, &result); err != nil {
		return DispatchResult{}, fmt.Errorf("decode dispatch %q: %w", request.Event, err)
	}
	if err := result.Validate(); err != nil {
		return DispatchResult{}, fmt.Errorf("validate dispatch %q: %w", request.Event, err)
	}
	return result, nil
}

// ExecuteSaga sends a versioned, result-capable workflow request to the Lua
// orchestrator. The returned result is safe to expose to an application only
// after its validation confirms the declared durable effect state.
func (c *Client) ExecuteSaga(request SagaRequest) (SagaResult, error) {
	if c == nil {
		return SagaResult{}, fmt.Errorf("workflow client is nil")
	}
	if err := validateName("orchestrator cell name", c.Name); err != nil {
		return SagaResult{}, err
	}
	if err := request.Validate(); err != nil {
		return SagaResult{}, fmt.Errorf("validate saga request: %w", err)
	}
	encoded, err := msgpack.Marshal(request)
	if err != nil {
		return SagaResult{}, fmt.Errorf("encode saga request: %w", err)
	}
	caller := c.call
	if caller == nil {
		caller = defaultPulpCall
	}
	response, err := caller(c.Name, FnExecuteSaga, encoded)
	if err != nil {
		return SagaResult{}, fmt.Errorf("execute saga %q: %w", request.Name, err)
	}
	if len(response) == 0 {
		return SagaResult{}, fmt.Errorf("execute saga %q returned an empty response", request.Name)
	}
	var result SagaResult
	if err := msgpack.Unmarshal(response, &result); err != nil {
		return SagaResult{}, fmt.Errorf("decode saga %q: %w", request.Name, err)
	}
	if err := result.Validate(); err != nil {
		return SagaResult{}, fmt.Errorf("validate saga %q: %w", request.Name, err)
	}
	if result.Name != request.Name || result.RequestID != request.RequestID || result.IdempotencyKey != request.IdempotencyKey {
		return SagaResult{}, fmt.Errorf("saga %q response identity does not match request", request.Name)
	}
	return result, nil
}
