package effect

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestExecuteServerMutationHostV4PreservesExactReceipts(t *testing.T) {
	previous := serverMutationHostExecuteWire
	t.Cleanup(func() { serverMutationHostExecuteWire = previous })
	serverMutationHostExecuteWire = func(request []byte) ([]byte, uint32) {
		var value ServerMutationHostRequestV4
		decoder := msgpack.NewDecoder(bytes.NewReader(request))
		decoder.DisallowUnknownFields(true)
		if err := decoder.Decode(&value); err != nil {
			t.Fatal(err)
		}
		if value.Version != ServerMutationHostContractV4 ||
			value.Owner != "runtime-control" ||
			!bytes.Equal(value.Claim, []byte("exact-claim")) {
			t.Fatalf("request = %#v", value)
		}
		response, err := msgpack.Marshal(ServerMutationHostResultV4{
			Version:          ServerMutationHostContractV4,
			Owner:            value.Owner,
			GenericReceipt:   []byte("generic"),
			OperationReceipt: []byte("operation"),
		})
		if err != nil {
			t.Fatal(err)
		}
		return response, 0
	}
	result, err := ExecuteServerMutationHostV4("runtime-control", []byte("exact-claim"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result.GenericReceipt, []byte("generic")) ||
		!bytes.Equal(result.OperationReceipt, []byte("operation")) {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecuteServerMutationHostV4FailsClosed(t *testing.T) {
	for _, owner := range []string{"", "fleet", " runtime-control"} {
		if _, err := ExecuteServerMutationHostV4(owner, []byte("claim")); err == nil {
			t.Fatalf("owner %q succeeded", owner)
		}
	}
	previous := serverMutationHostExecuteWire
	t.Cleanup(func() { serverMutationHostExecuteWire = previous })
	serverMutationHostExecuteWire = func([]byte) ([]byte, uint32) {
		response, _ := msgpack.Marshal(ServerMutationHostResultV4{
			Version:          ServerMutationHostContractV4,
			Owner:            "workload-provisioning",
			GenericReceipt:   []byte("generic"),
			OperationReceipt: []byte("operation"),
		})
		return response, 0
	}
	if _, err := ExecuteServerMutationHostV4(
		"runtime-control", []byte("claim"),
	); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("mismatched response error = %v", err)
	}
}
