// Package workflow defines the transport-neutral wire shared by application
// shells and Pulp orchestration cells.
package workflow

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/vmihailenco/msgpack/v5"
)

// FnDispatch is the stable sibling function exposed by an orchestration cell.
const FnDispatch = "orchestrator.dispatch"

const maxNameLength = 256

// DispatchRequest asks the application orchestrator to handle one named event.
// Payload must be MessagePack-compatible.
type DispatchRequest struct {
	Event   string `msgpack:"event"`
	Payload any    `msgpack:"payload,omitempty"`
}

// Action is a declarative command or event returned by the orchestrator.
// The caller remains responsible for decoding, authorizing, and applying it.
type Action struct {
	Name    string `msgpack:"name"`
	Payload any    `msgpack:"payload,omitempty"`
}

// DispatchResult is returned by FnDispatch. Commands and events are
// declarative: callers apply or publish them only after the sibling call has
// returned successfully.
type DispatchResult struct {
	Value    any      `msgpack:"value,omitempty"`
	Commands []Action `msgpack:"commands,omitempty"`
	Events   []Action `msgpack:"events,omitempty"`
}

// Validate rejects malformed requests before a sibling call is attempted.
func (r DispatchRequest) Validate() error {
	if err := validateName("event", r.Event); err != nil {
		return err
	}
	return nil
}

// Validate rejects malformed actions before a caller touches state or
// publishes an event.
func (a Action) Validate() error {
	return validateName("action", a.Name)
}

// Validate rejects a malformed orchestration response. Payload-specific
// validation remains with the typed command or event owner.
func (r DispatchResult) Validate() error {
	for i, command := range r.Commands {
		if err := command.Validate(); err != nil {
			return fmt.Errorf("command %d: %w", i, err)
		}
	}
	for i, event := range r.Events {
		if err := event.Validate(); err != nil {
			return fmt.Errorf("event %d: %w", i, err)
		}
	}
	return nil
}

// DecodeValue converts a generic MessagePack-compatible result value into a
// concrete caller-owned type. This is the typed bridge used after Lua returns
// maps and arrays.
func DecodeValue[T any](result DispatchResult) (T, error) {
	var zero T
	if result.Value == nil {
		return zero, fmt.Errorf("workflow result has no value")
	}
	encoded, err := msgpack.Marshal(result.Value)
	if err != nil {
		return zero, fmt.Errorf("encode workflow value: %w", err)
	}
	var value T
	if err := msgpack.Unmarshal(encoded, &value); err != nil {
		return zero, fmt.Errorf("decode workflow value: %w", err)
	}
	return value, nil
}

func validateName(kind, name string) error {
	if name == "" {
		return fmt.Errorf("%s is required", kind)
	}
	if strings.TrimSpace(name) != name {
		return fmt.Errorf("%s must not have surrounding whitespace", kind)
	}
	if len(name) > maxNameLength {
		return fmt.Errorf("%s exceeds %d bytes", kind, maxNameLength)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return fmt.Errorf("%s contains a control character", kind)
		}
	}
	return nil
}
