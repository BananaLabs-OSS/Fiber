package docker

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/vmihailenco/msgpack/v5"
)

// getOwnedRequest deliberately carries only an application-local logical
// name. The host applies the caller's immutable application/cell scope and
// returns the canonical container identity it owns.
type getOwnedRequest struct {
	LogicalName string `msgpack:"logical_name"`
}

// GetOwned resolves one application-local logical name through the scoped
// spawn.docker host boundary. It never constructs a host name, searches by
// suffix, or guesses an ownership prefix. The returned Server and ID are the
// host's canonical scoped identity and are safe to persist for replay.
func GetOwned(logicalName string) (*Server, error) {
	if err := validateOwnedLogicalName(logicalName); err != nil {
		return nil, err
	}
	request, err := msgpack.Marshal(getOwnedRequest{LogicalName: logicalName})
	if err != nil {
		return nil, fmt.Errorf("docker_get_owned: encode request: %w", err)
	}
	response, code := dockerGetOwnedWire(request)
	if err := codeToError("docker_get_owned", code); err != nil {
		return nil, err
	}
	if len(response) == 0 {
		return nil, fmt.Errorf("docker_get_owned: empty response")
	}

	var server Server
	if err := msgpack.Unmarshal(response, &server); err != nil {
		return nil, fmt.Errorf("docker_get_owned: decode response: %w", err)
	}
	if err := validateOwnedServer(server); err != nil {
		return nil, err
	}
	return &server, nil
}

func validateOwnedLogicalName(logicalName string) error {
	if logicalName == "" || strings.TrimSpace(logicalName) != logicalName {
		return fmt.Errorf("docker_get_owned: logical_name: %w", ErrInvalidRequest)
	}
	for _, r := range logicalName {
		if unicode.IsControl(r) {
			return fmt.Errorf("docker_get_owned: logical_name: %w", ErrInvalidRequest)
		}
	}
	return nil
}

func validateOwnedServer(server Server) error {
	if strings.TrimSpace(server.ID) == "" || strings.TrimSpace(server.Name) == "" {
		return fmt.Errorf("docker_get_owned: host returned an empty canonical server identity")
	}
	return nil
}
