package effect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func validLargeExactObjectValidationCommand() StorageLargeExactObjectValidationCommandV1 {
	return StorageLargeExactObjectValidationCommandV1{
		ContractVersion: StorageLargeExactObjectValidationContractV1,
		Claim: StorageLargeExactObjectValidationClaimV1{
			Version: StorageLargeExactObjectValidationClaimVersionV1,
			ID:      "claim-1",
			Digest:  strings.Repeat("a", 64),
		},
		ObjectNamespace:    "application-uploads.v1",
		ObjectKey:          "uploads/claim-1/archive.zip",
		ExpectedGeneration: "etag-1",
		ContentLength:      StorageLargeExactObjectValidationMaxContentBytesV1,
		ExpectedSHA256:     strings.Repeat("b", 64),
		ValidatorPolicy:    "application.archive.v1",
		Limits:             StorageLargeExactObjectValidationFixedLimitsV1(),
	}
}

func validLargeExactObjectValidationResult(
	command StorageLargeExactObjectValidationCommandV1,
) StorageLargeExactObjectValidationResultV1 {
	return StorageLargeExactObjectValidationResultV1{
		ContractVersion:       command.ContractVersion,
		Claim:                 command.Claim,
		ObjectNamespace:       command.ObjectNamespace,
		ObjectKey:             command.ObjectKey,
		StorageGeneration:     command.ExpectedGeneration,
		ObservedContentLength: command.ContentLength,
		ExpectedSHA256:        command.ExpectedSHA256,
		ObservedSHA256:        command.ExpectedSHA256,
		ValidatorPolicy:       command.ValidatorPolicy,
		Valid:                 true,
		Object: &StorageObjectGenerationRefV1{
			Version:    StorageObjectGenerationRefVersionV1,
			Namespace:  command.ObjectNamespace,
			Key:        command.ObjectKey,
			Generation: 1,
			SHA256:     command.ExpectedSHA256,
			SizeBytes:  command.ContentLength,
		},
	}
}

func TestLargeExactObjectValidationIsAdditiveAndClaimBound(t *testing.T) {
	if StorageArtifactZIPValidationMaxContentBytesV1 != 50<<20 {
		t.Fatalf("small-object limit changed: %d", StorageArtifactZIPValidationMaxContentBytesV1)
	}
	if StorageExactObjectUploadMaxBytes != 64<<20 {
		t.Fatalf("single-PUT upload limit changed: %d", StorageExactObjectUploadMaxBytes)
	}
	if StorageLargeExactObjectValidationMaxContentBytesV1 != 500<<20 {
		t.Fatalf("large validation limit = %d", StorageLargeExactObjectValidationMaxContentBytesV1)
	}

	command := validLargeExactObjectValidationCommand()
	previous := storageArtifactZIPValidateWire
	t.Cleanup(func() { storageArtifactZIPValidateWire = previous })
	storageArtifactZIPValidateWire = func(wire []byte) ([]byte, uint32) {
		decoded, err := decodeStorageLargeExactObjectValidationCommandV1(wire)
		if err != nil {
			t.Fatalf("decode command: %v", err)
		}
		if decoded != command {
			t.Fatalf("decoded command = %#v", decoded)
		}
		response, err := msgpack.Marshal(validLargeExactObjectValidationResult(decoded))
		if err != nil {
			t.Fatal(err)
		}
		return response, 0
	}
	result, err := ValidateLargeExactObject(command)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid || result.Object == nil || result.Object.Generation != 1 {
		t.Fatalf("terminal result = %#v", result)
	}
}

func TestLargeExactObjectValidationRejectsClaimTamperAndAuthority(t *testing.T) {
	valid := validLargeExactObjectValidationCommand()
	mutations := []func(*StorageLargeExactObjectValidationCommandV1){
		func(value *StorageLargeExactObjectValidationCommandV1) { value.Claim.ID = "" },
		func(value *StorageLargeExactObjectValidationCommandV1) { value.Claim.Digest = strings.Repeat("A", 64) },
		func(value *StorageLargeExactObjectValidationCommandV1) { value.ObjectNamespace = "" },
		func(value *StorageLargeExactObjectValidationCommandV1) { value.ObjectKey = "../archive.zip" },
		func(value *StorageLargeExactObjectValidationCommandV1) {
			value.ExpectedGeneration = StorageObjectGenerationAbsent
		},
		func(value *StorageLargeExactObjectValidationCommandV1) { value.ContentLength++ },
		func(value *StorageLargeExactObjectValidationCommandV1) {
			value.ExpectedSHA256 = "sha256:" + strings.Repeat("b", 64)
		},
		func(value *StorageLargeExactObjectValidationCommandV1) { value.ValidatorPolicy = "" },
		func(value *StorageLargeExactObjectValidationCommandV1) { value.Limits.MaxEntries++ },
	}
	for index, mutate := range mutations {
		command := valid
		mutate(&command)
		if err := command.Validate(); err == nil {
			t.Fatalf("mutation %d succeeded: %#v", index, command)
		}
	}

	base, err := msgpack.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := msgpack.Unmarshal(base, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"bucket", "endpoint", "headers", "path", "credentials"} {
		decoded[field] = "caller-authority"
		wire, err := msgpack.Marshal(decoded)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := decodeStorageLargeExactObjectValidationCommandV1(wire); err == nil {
			t.Fatalf("unknown authority %q succeeded", field)
		}
		delete(decoded, field)
	}
}

func TestLargeExactObjectValidationRejectsReplayEvidenceMutation(t *testing.T) {
	command := validLargeExactObjectValidationCommand()
	previous := storageArtifactZIPValidateWire
	t.Cleanup(func() { storageArtifactZIPValidateWire = previous })
	mutations := []func(*StorageLargeExactObjectValidationResultV1){
		func(value *StorageLargeExactObjectValidationResultV1) { value.Claim.ID = "other-claim" },
		func(value *StorageLargeExactObjectValidationResultV1) { value.Claim.Digest = strings.Repeat("c", 64) },
		func(value *StorageLargeExactObjectValidationResultV1) { value.ObjectKey = "uploads/other/archive.zip" },
		func(value *StorageLargeExactObjectValidationResultV1) { value.StorageGeneration = "etag-2" },
		func(value *StorageLargeExactObjectValidationResultV1) { value.ExpectedSHA256 = strings.Repeat("c", 64) },
		func(value *StorageLargeExactObjectValidationResultV1) { value.ValidatorPolicy = "other-policy" },
	}
	for index, mutate := range mutations {
		storageArtifactZIPValidateWire = func([]byte) ([]byte, uint32) {
			result := validLargeExactObjectValidationResult(command)
			mutate(&result)
			wire, err := msgpack.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			return wire, 0
		}
		if _, err := ValidateLargeExactObject(command); err == nil {
			t.Fatalf("mutated replay evidence %d succeeded", index)
		}
	}
}

func TestLargeExactObjectGenericSourceHasNoApplicationVocabulary(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("storage_large_exact_object_validation.go"))
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(source))
	for _, forbidden := range []string{"minecraft", "evolution", "world-archive", "sessions"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("generic contract contains application vocabulary %q", forbidden)
		}
	}
}

func FuzzDecodeStorageLargeExactObjectValidationCommand(f *testing.F) {
	valid, err := msgpack.Marshal(validLargeExactObjectValidationCommand())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(valid)
	f.Add([]byte{0x81, 0xa1, 'x', 0xc3})
	f.Fuzz(func(t *testing.T, wire []byte) {
		command, err := decodeStorageLargeExactObjectValidationCommandV1(wire)
		if err == nil {
			if validateErr := command.Validate(); validateErr != nil {
				t.Fatalf("decoder admitted invalid command: %v", validateErr)
			}
			if command.ContentLength > StorageLargeExactObjectValidationMaxContentBytesV1 {
				t.Fatalf("decoder admitted unbounded content length")
			}
		}
	})
}
