package effect

import (
	"errors"
	"testing"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

func TestStorageExactObjectDownloadReferenceContract(t *testing.T) {
	command := StorageExactObjectDownloadReferenceCommand{
		ContractVersion: StorageExactObjectDownloadReferenceContractV1,
		ObjectKey:       "uploads/order-1/bedrock.zip", ExpectedETag: "etag-1", ExpiresAtUnix: 123,
	}
	if err := command.Validate(); err != nil {
		t.Fatal(err)
	}
	old := storageExactObjectDownloadReferenceWire
	t.Cleanup(func() { storageExactObjectDownloadReferenceWire = old })
	storageExactObjectDownloadReferenceWire = func(wire []byte) ([]byte, uint32) {
		var got StorageExactObjectDownloadReferenceCommand
		if err := msgpack.Unmarshal(wire, &got); err != nil || got != command {
			t.Errorf("command = %#v, %v", got, err)
			return nil, 4
		}
		result, err := msgpack.Marshal(StorageExactObjectDownloadReferenceResult{
			ContractVersion: got.ContractVersion, ObjectKey: got.ObjectKey, ExpectedETag: got.ExpectedETag,
			PublicURL: "https://downloads.example.test/uploads/order-1/bedrock.zip", ExpiresAtUnix: got.ExpiresAtUnix,
		})
		if err != nil {
			t.Fatal(err)
		}
		return result, 0
	}
	result, err := DownloadReferenceExactObject(command)
	if err != nil || result.PublicURL == "" {
		t.Fatalf("result = %#v, %v", result, err)
	}
}

func TestStorageExactObjectDownloadReferenceFailsClosed(t *testing.T) {
	valid := StorageExactObjectDownloadReferenceCommand{ContractVersion: StorageExactObjectDownloadReferenceContractV1, ObjectKey: "uploads/a.zip", ExpectedETag: "etag-1", ExpiresAtUnix: 1}
	for _, mutate := range []func(*StorageExactObjectDownloadReferenceCommand){
		func(c *StorageExactObjectDownloadReferenceCommand) { c.ContractVersion = "v2" },
		func(c *StorageExactObjectDownloadReferenceCommand) { c.ObjectKey = "../a.zip" },
		func(c *StorageExactObjectDownloadReferenceCommand) { c.ExpectedETag = StorageObjectGenerationAbsent },
		func(c *StorageExactObjectDownloadReferenceCommand) { c.ExpiresAtUnix = 0 },
	} {
		candidate := valid
		mutate(&candidate)
		if err := candidate.Validate(); err == nil {
			t.Fatalf("invalid command accepted: %#v", candidate)
		}
	}
	old := storageExactObjectDownloadReferenceWire
	t.Cleanup(func() { storageExactObjectDownloadReferenceWire = old })
	storageExactObjectDownloadReferenceWire = func([]byte) ([]byte, uint32) { return nil, 99 }
	if _, err := DownloadReferenceExactObject(valid); !errors.Is(err, pulp.ErrCapabilityUnavailable) {
		t.Fatalf("capability error = %v", err)
	}
}
