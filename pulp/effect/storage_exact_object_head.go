package effect

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/vmihailenco/msgpack/v5"
)

// StorageExactObjectHeadPayload requests inventory for the owner-derived,
// application-scoped ExactKey. Hosts must use the key verbatim and must not
// infer a bucket prefix or perform a list operation.
type StorageExactObjectHeadPayload struct {
	ExactKey    string `msgpack:"exact_key"`
	AllowAbsent bool   `msgpack:"allow_absent"`
}

// StorageExactObjectHeadResult is the durable generation inventory for the
// requested exact object. Absent objects have generation zero; present objects
// have a positive generation.
type StorageExactObjectHeadResult struct {
	ExactKey   string `msgpack:"exact_key"`
	Generation int64  `msgpack:"generation"`
	Absent     bool   `msgpack:"absent"`
}

func (p StorageExactObjectHeadPayload) Validate() error {
	return validateExactObjectKey("storage exact object key", p.ExactKey)
}

func (r StorageExactObjectHeadResult) Validate() error {
	if err := validateExactObjectKey("storage exact object result key", r.ExactKey); err != nil {
		return err
	}
	if r.Absent {
		if r.Generation != 0 {
			return fmt.Errorf("absent storage exact object result must have generation zero")
		}
		return nil
	}
	if r.Generation <= 0 {
		return fmt.Errorf("present storage exact object result must have a positive generation")
	}
	return nil
}

// validateExactObjectKey admits a single relative object name. It rejects
// traversal and separator ambiguity so the host cannot reinterpret an
// application-owned exact key as a filesystem path or a different object.
func validateExactObjectKey(label, key string) error {
	if err := validateField(label, key); err != nil {
		return err
	}
	if strings.HasPrefix(key, "/") || strings.Contains(key, "\\") {
		return fmt.Errorf("%s must be a relative slash-delimited key", label)
	}
	for _, segment := range strings.Split(key, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("%s contains an unsafe path segment", label)
		}
	}
	return nil
}

func decodeStorageExactObjectHeadPayload(raw msgpack.RawMessage) (StorageExactObjectHeadPayload, error) {
	var payload StorageExactObjectHeadPayload
	decoder := msgpack.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&payload); err != nil {
		return payload, fmt.Errorf("decode storage exact object head payload: %w", err)
	}
	return payload, nil
}

func decodeStorageExactObjectHeadResult(raw msgpack.RawMessage) (StorageExactObjectHeadResult, error) {
	var result StorageExactObjectHeadResult
	decoder := msgpack.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("decode storage exact object head result: %w", err)
	}
	return result, nil
}
