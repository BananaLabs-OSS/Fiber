package pulp

import (
	"errors"
	"strings"
	"testing"
)

func TestRecordCallErrorBoundsAndClearsDiagnostic(t *testing.T) {
	recordCallError(errors.New(strings.Repeat("x", maxInitErrorBytes+64)))
	if got := len(lastCallError); got != maxInitErrorBytes {
		t.Fatalf("diagnostic length = %d, want %d", got, maxInitErrorBytes)
	}
	if got := string(lastCallError[len(lastCallError)-3:]); got != "..." {
		t.Fatalf("diagnostic suffix = %q, want ellipsis", got)
	}

	recordCallError(nil)
	if lastCallError != nil {
		t.Fatalf("diagnostic was not cleared: %q", lastCallError)
	}
}
