// Package cellconfig decodes the MessagePack config payload Pulp passes to a
// cell's init hook into application structs that use JSON field tags.
package cellconfig

import (
	"encoding/json"

	"github.com/vmihailenco/msgpack/v5"
)

// Decode converts a Pulp manifest config payload into out. Pulp's manifest
// encoder produces a map; the JSON bridge preserves the field-tag behavior the
// hosting cells historically implemented themselves.
func Decode(data []byte, out any) error {
	var raw map[string]any
	if err := msgpack.Unmarshal(data, &raw); err != nil {
		return err
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, out)
}
