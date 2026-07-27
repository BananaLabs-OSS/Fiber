package effect

import (
	"bytes"
	"fmt"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	ServerMutationHostContractV4 = "server-mutation-host.v4"
	serverMutationHostMaxWire    = 1 << 20
)

type ServerMutationHostRequestV4 struct {
	Version string `msgpack:"version"`
	Owner   string `msgpack:"owner"`
	Claim   []byte `msgpack:"claim"`
}

type ServerMutationHostResultV4 struct {
	Version          string `msgpack:"version"`
	Owner            string `msgpack:"owner"`
	GenericReceipt   []byte `msgpack:"generic_receipt"`
	OperationReceipt []byte `msgpack:"operation_receipt"`
}

// ExecuteServerMutationHostV4 crosses only the exact leased-claim host ABI.
// The guest cannot select a URL, credential, command, or operation handler.
func ExecuteServerMutationHostV4(
	owner string,
	claim []byte,
) (ServerMutationHostResultV4, error) {
	if owner != "runtime-control" && owner != "workload-provisioning" {
		return ServerMutationHostResultV4{}, fmt.Errorf("server mutation host: owner is not allowed")
	}
	if len(claim) == 0 || len(claim) > serverMutationHostMaxWire {
		return ServerMutationHostResultV4{}, fmt.Errorf("server mutation host: exact claim is empty or exceeds limit")
	}
	request, err := msgpack.Marshal(ServerMutationHostRequestV4{
		Version: ServerMutationHostContractV4,
		Owner:   owner,
		Claim:   append([]byte(nil), claim...),
	})
	if err != nil {
		return ServerMutationHostResultV4{}, fmt.Errorf("server mutation host: encode request: %w", err)
	}
	response, code := serverMutationHostExecuteWire(request)
	if err := serverMutationHostCodeError(code); err != nil {
		return ServerMutationHostResultV4{}, err
	}
	if len(response) == 0 || len(response) > serverMutationHostMaxWire {
		return ServerMutationHostResultV4{}, fmt.Errorf("server mutation host: response is empty or exceeds limit")
	}
	var result ServerMutationHostResultV4
	decoder := msgpack.NewDecoder(bytes.NewReader(response))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return ServerMutationHostResultV4{}, fmt.Errorf("server mutation host: decode response: %w", err)
	}
	if result.Version != ServerMutationHostContractV4 ||
		result.Owner != owner ||
		len(result.GenericReceipt) == 0 ||
		len(result.OperationReceipt) == 0 ||
		len(result.GenericReceipt) > serverMutationHostMaxWire ||
		len(result.OperationReceipt) > serverMutationHostMaxWire {
		return ServerMutationHostResultV4{}, fmt.Errorf("server mutation host: response identity is invalid")
	}
	result.GenericReceipt = append([]byte(nil), result.GenericReceipt...)
	result.OperationReceipt = append([]byte(nil), result.OperationReceipt...)
	return result, nil
}

func serverMutationHostCodeError(code uint32) error {
	switch code {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("server mutation host: empty request")
	case 2:
		return fmt.Errorf("server mutation host: request memory read failed")
	case 3:
		return fmt.Errorf("server mutation host: request decode failed")
	case 4:
		return fmt.Errorf("server mutation host: invalid exact claim")
	case 5:
		return fmt.Errorf("server mutation host: execution failed")
	case 6:
		return fmt.Errorf("server mutation host: response allocation or write failed")
	case 99:
		return fmt.Errorf("server mutation host: %w", pulp.ErrCapabilityUnavailable)
	default:
		return fmt.Errorf("server mutation host: host code %d", code)
	}
}
