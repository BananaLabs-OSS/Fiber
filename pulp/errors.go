package pulp

import "errors"

// ErrCapabilityUnavailable is returned by every capability wrapper when
// the host stubbed the capability — i.e. the cell did not declare it
// in its manifest's `capabilities` list. It maps to host return code 99.
//
// Callers can branch on it with errors.Is to degrade gracefully:
//
//	if _, err := pulp.SQLite.Exec(q); errors.Is(err, pulp.ErrCapabilityUnavailable) {
//	    // storage.sqlite not declared — fall back or skip
//	}
var ErrCapabilityUnavailable = errors.New("pulp: capability unavailable (declare it in cell manifest)")

// ErrNotFound is returned by capability wrappers (fs, s3, docker, etc.)
// when the named resource does not exist. Mapped from host code 6.
var ErrNotFound = errors.New("pulp: not found")
