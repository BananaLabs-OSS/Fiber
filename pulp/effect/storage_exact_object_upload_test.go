package effect

import (
	"errors"
	"testing"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

func TestStorageExactObjectPublicUploadContract(t *testing.T) {
	presign := StorageExactObjectPresignPutCommand{
		ExactKey: "sessions/uploads/order-1/world.zip", UploadID: "upload-1",
		ExpectedGeneration: StorageObjectGenerationAbsent, ContentLength: 1024,
		ContentType: "application/zip", TTLSec: 300,
	}
	if err := presign.Validate(); err != nil {
		t.Fatalf("presign Validate: %v", err)
	}
	validate := StorageExactObjectValidatePutCommand{
		ExactKey: presign.ExactKey, UploadID: presign.UploadID,
		ContentLength: presign.ContentLength, ContentType: presign.ContentType,
	}
	if err := validate.Validate(); err != nil {
		t.Fatalf("validate Validate: %v", err)
	}
	deleteCommand := StorageExactObjectDeleteCommand{ExactKey: presign.ExactKey, ExpectedGeneration: "etag-1"}
	if err := deleteCommand.Validate(); err != nil {
		t.Fatalf("delete Validate: %v", err)
	}

	oldPresign, oldValidate, oldDelete := storageExactObjectPresignPutWire, storageExactObjectValidatePutWire, storageExactObjectDeleteWire
	t.Cleanup(func() {
		storageExactObjectPresignPutWire, storageExactObjectValidatePutWire, storageExactObjectDeleteWire = oldPresign, oldValidate, oldDelete
	})
	storageExactObjectPresignPutWire = func(wire []byte) ([]byte, uint32) {
		var command StorageExactObjectPresignPutCommand
		if err := msgpack.Unmarshal(wire, &command); err != nil || command != presign {
			t.Errorf("presign command = %#v, %v", command, err)
			return nil, 4
		}
		result, err := msgpack.Marshal(StorageExactObjectPresignPutResult{
			ExactKey: command.ExactKey, UploadID: command.UploadID, ExpectedGeneration: command.ExpectedGeneration,
			ContentLength: command.ContentLength, ContentType: command.ContentType,
			URL: "https://object.test/upload", ExpiresAtUnix: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		return result, 0
	}
	storageExactObjectValidatePutWire = func(wire []byte) ([]byte, uint32) {
		var command StorageExactObjectValidatePutCommand
		if err := msgpack.Unmarshal(wire, &command); err != nil || command != validate {
			t.Errorf("validation command = %#v, %v", command, err)
			return nil, 4
		}
		result, err := msgpack.Marshal(StorageExactObjectValidatePutResult{
			ExactKey: command.ExactKey, UploadID: command.UploadID, Generation: "etag-1",
			ContentLength: command.ContentLength, ContentType: command.ContentType,
		})
		if err != nil {
			t.Fatal(err)
		}
		return result, 0
	}
	storageExactObjectDeleteWire = func(wire []byte) ([]byte, uint32) {
		var command StorageExactObjectDeleteCommand
		if err := msgpack.Unmarshal(wire, &command); err != nil || command != deleteCommand {
			t.Errorf("delete command = %#v, %v", command, err)
			return nil, 4
		}
		result, err := msgpack.Marshal(StorageExactObjectDeleteResult{ExactKey: command.ExactKey, ExpectedGeneration: command.ExpectedGeneration})
		if err != nil {
			t.Fatal(err)
		}
		return result, 0
	}
	if got, err := PresignExactObjectPut(presign); err != nil || got.URL == "" {
		t.Fatalf("PresignExactObjectPut = %#v, %v", got, err)
	}
	if got, err := ValidateExactObjectPut(validate); err != nil || got.Generation != "etag-1" {
		t.Fatalf("ValidateExactObjectPut = %#v, %v", got, err)
	}
	if got, err := DeleteExactObject(deleteCommand); err != nil || got != (StorageExactObjectDeleteResult{ExactKey: deleteCommand.ExactKey, ExpectedGeneration: deleteCommand.ExpectedGeneration}) {
		t.Fatalf("DeleteExactObject = %#v, %v", got, err)
	}
}

func TestStorageExactObjectDeleteFailsClosed(t *testing.T) {
	valid := StorageExactObjectDeleteCommand{ExactKey: "uploads/a.zip", ExpectedGeneration: "etag-1"}
	for _, mutate := range []func(*StorageExactObjectDeleteCommand){
		func(c *StorageExactObjectDeleteCommand) { c.ExactKey = "../other.zip" },
		func(c *StorageExactObjectDeleteCommand) { c.ExpectedGeneration = "" },
		func(c *StorageExactObjectDeleteCommand) { c.ExpectedGeneration = StorageObjectGenerationAbsent },
	} {
		command := valid
		mutate(&command)
		if err := command.Validate(); err == nil {
			t.Fatalf("unsafe delete command validated: %#v", command)
		}
	}
	old := storageExactObjectDeleteWire
	t.Cleanup(func() { storageExactObjectDeleteWire = old })
	storageExactObjectDeleteWire = func([]byte) ([]byte, uint32) {
		wire, _ := msgpack.Marshal(StorageExactObjectDeleteResult{ExactKey: "uploads/other.zip", ExpectedGeneration: valid.ExpectedGeneration})
		return wire, 0
	}
	if _, err := DeleteExactObject(valid); err == nil {
		t.Fatal("mismatched delete result was accepted")
	}
	storageExactObjectDeleteWire = func([]byte) ([]byte, uint32) { return nil, 11 }
	if _, err := DeleteExactObject(valid); err == nil {
		t.Fatal("generation mismatch code was accepted")
	}
	storageExactObjectDeleteWire = func([]byte) ([]byte, uint32) { return nil, 99 }
	if _, err := DeleteExactObject(valid); !errors.Is(err, pulp.ErrCapabilityUnavailable) {
		t.Fatalf("capability error = %v", err)
	}
}

func TestStorageExactObjectPublicUploadRejectsUnsafeOrUnboundedCommands(t *testing.T) {
	valid := StorageExactObjectPresignPutCommand{ExactKey: "uploads/a.zip", UploadID: "upload-a", ExpectedGeneration: StorageObjectGenerationAbsent, ContentLength: 1, ContentType: "application/zip", TTLSec: 1}
	for _, mutate := range []func(*StorageExactObjectPresignPutCommand){
		func(c *StorageExactObjectPresignPutCommand) { c.ExactKey = "uploads/" },
		func(c *StorageExactObjectPresignPutCommand) { c.ExactKey = "../other/object" },
		func(c *StorageExactObjectPresignPutCommand) { c.ContentLength = 0 },
		func(c *StorageExactObjectPresignPutCommand) { c.ContentLength = StorageExactObjectUploadMaxBytes + 1 },
		func(c *StorageExactObjectPresignPutCommand) { c.ContentType = "application/zip; charset=utf-8" },
		func(c *StorageExactObjectPresignPutCommand) { c.TTLSec = 901 },
		func(c *StorageExactObjectPresignPutCommand) { c.ExpectedGeneration = "" },
	} {
		command := valid
		mutate(&command)
		if err := command.Validate(); err == nil {
			t.Fatalf("unsafe command validated: %#v", command)
		}
	}
}

func TestStorageExactObjectPublicUploadFailsClosedForMismatchedResultAndCapability(t *testing.T) {
	command := StorageExactObjectPresignPutCommand{ExactKey: "uploads/a.zip", UploadID: "upload-a", ExpectedGeneration: StorageObjectGenerationAbsent, ContentLength: 1, ContentType: "application/zip", TTLSec: 1}
	old := storageExactObjectPresignPutWire
	t.Cleanup(func() { storageExactObjectPresignPutWire = old })
	storageExactObjectPresignPutWire = func([]byte) ([]byte, uint32) {
		wire, _ := msgpack.Marshal(StorageExactObjectPresignPutResult{ExactKey: "uploads/other.zip", UploadID: command.UploadID, ExpectedGeneration: command.ExpectedGeneration, ContentLength: command.ContentLength, ContentType: command.ContentType, URL: "https://object.test/upload", ExpiresAtUnix: 1})
		return wire, 0
	}
	if _, err := PresignExactObjectPut(command); err == nil {
		t.Fatal("mismatched exact key was accepted")
	}
	storageExactObjectPresignPutWire = func([]byte) ([]byte, uint32) { return nil, 99 }
	if _, err := PresignExactObjectPut(command); !errors.Is(err, pulp.ErrCapabilityUnavailable) {
		t.Fatalf("capability error = %v", err)
	}
}
