//go:build !wasip1

package workflow

import "fmt"

func defaultPulpCall(_, _ string, _ []byte) ([]byte, error) {
	return nil, fmt.Errorf("workflow client requires a Pulp WASI host")
}
