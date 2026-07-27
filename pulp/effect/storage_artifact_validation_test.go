package effect

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

func validStorageArtifactZIPValidationCommandV1() StorageArtifactZIPValidationCommandV1 {
	return StorageArtifactZIPValidationCommandV1{
		ContractVersion:    StorageArtifactValidationContractV1,
		UploadID:           "upload-1",
		ObjectKey:          "sessions/app-1/artifacts/world.zip",
		ExpectedGeneration: "etag-1",
		ContentLength:      1024,
		Purpose:            StorageArtifactPurposeDatapackV1,
		ValidatorVersion:   StorageArtifactZIPValidatorVersionV1,
		Limits:             StorageArtifactZIPValidationFixedLimitsV1(),
	}
}

func validStorageArtifactZIPValidationResultV1(command StorageArtifactZIPValidationCommandV1) StorageArtifactZIPValidationResultV1 {
	return StorageArtifactZIPValidationResultV1{
		ContractVersion:       command.ContractVersion,
		UploadID:              command.UploadID,
		ObjectKey:             command.ObjectKey,
		Generation:            command.ExpectedGeneration,
		Purpose:               command.Purpose,
		Valid:                 true,
		ObservedContentLength: command.ContentLength,
		SHA256:                strings.Repeat("a", 64),
		ValidatorVersion:      command.ValidatorVersion,
	}
}

func TestStorageArtifactZIPValidationContract(t *testing.T) {
	if StorageArtifactValidationCapability != "storage.s3.artifact-validation.v1" {
		t.Fatalf("capability = %q", StorageArtifactValidationCapability)
	}
	if StorageArtifactValidationContractV1 != StorageArtifactValidationCapability {
		t.Fatalf("contract = %q", StorageArtifactValidationContractV1)
	}
	if StorageExactObjectArtifactZIPValidateImport != "s3_exact_object_validate_artifact_zip" {
		t.Fatalf("import = %q", StorageExactObjectArtifactZIPValidateImport)
	}
	if StorageArtifactZIPValidatorVersionV1 != "zip-artifact-validator.v1" {
		t.Fatalf("validator version = %q", StorageArtifactZIPValidatorVersionV1)
	}
	wantLimits := StorageArtifactZIPValidationLimitsV1{
		MaxEntries:                4096,
		MaxTotalUncompressedBytes: 256 << 20,
		MaxEntryUncompressedBytes: 128 << 20,
		MaxCompressionRatio:       100,
		MaxManifestBytes:          1 << 20,
	}
	if got := StorageArtifactZIPValidationFixedLimitsV1(); got != wantLimits {
		t.Fatalf("fixed limits = %#v, want %#v", got, wantLimits)
	}
	if StorageArtifactZIPValidationMaxContentBytesV1 != 50<<20 {
		t.Fatalf("max content bytes = %d", StorageArtifactZIPValidationMaxContentBytesV1)
	}

	command := validStorageArtifactZIPValidationCommandV1()
	old := storageArtifactZIPValidateWire
	t.Cleanup(func() { storageArtifactZIPValidateWire = old })
	storageArtifactZIPValidateWire = func(request []byte) ([]byte, uint32) {
		decoded, err := decodeStorageArtifactZIPValidationCommandV1(request)
		if err != nil {
			t.Errorf("decode command: %v", err)
			return nil, 4
		}
		if !reflect.DeepEqual(decoded, command) {
			t.Errorf("command = %#v, want %#v", decoded, command)
			return nil, 4
		}
		wire, err := msgpack.Marshal(validStorageArtifactZIPValidationResultV1(decoded))
		if err != nil {
			t.Fatalf("marshal result: %v", err)
		}
		return wire, 0
	}

	result, err := ValidateExactObjectArtifactZIP(command)
	if err != nil {
		t.Fatalf("ValidateExactObjectArtifactZIP: %v", err)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result Validate: %v", err)
	}
}

func TestStorageArtifactZIPValidationAllowsOnlyDeclaredPurposes(t *testing.T) {
	for _, purpose := range []StorageArtifactPurposeV1{
		StorageArtifactPurposeDatapackV1,
		StorageArtifactPurposeBedrockResourceV1,
		StorageArtifactPurposeBedrockBehaviorV1,
	} {
		command := validStorageArtifactZIPValidationCommandV1()
		command.Purpose = purpose
		if err := command.Validate(); err != nil {
			t.Fatalf("purpose %q: %v", purpose, err)
		}
	}
}

func TestStorageArtifactZIPValidationRejectsUnsafeOrUnboundedCommands(t *testing.T) {
	valid := validStorageArtifactZIPValidationCommandV1()
	mutations := []func(*StorageArtifactZIPValidationCommandV1){
		func(c *StorageArtifactZIPValidationCommandV1) {
			c.ContractVersion = "storage.s3.artifact-validation.v2"
		},
		func(c *StorageArtifactZIPValidationCommandV1) { c.UploadID = "" },
		func(c *StorageArtifactZIPValidationCommandV1) { c.ObjectKey = "" },
		func(c *StorageArtifactZIPValidationCommandV1) { c.ObjectKey = "/sessions/world.zip" },
		func(c *StorageArtifactZIPValidationCommandV1) { c.ObjectKey = "sessions/../world.zip" },
		func(c *StorageArtifactZIPValidationCommandV1) { c.ObjectKey = `sessions\world.zip` },
		func(c *StorageArtifactZIPValidationCommandV1) { c.ObjectKey = "sessions/world.tar" },
		func(c *StorageArtifactZIPValidationCommandV1) { c.ExpectedGeneration = "" },
		func(c *StorageArtifactZIPValidationCommandV1) { c.ExpectedGeneration = StorageObjectGenerationAbsent },
		func(c *StorageArtifactZIPValidationCommandV1) { c.ContentLength = 0 },
		func(c *StorageArtifactZIPValidationCommandV1) {
			c.ContentLength = StorageArtifactZIPValidationMaxContentBytesV1 + 1
		},
		func(c *StorageArtifactZIPValidationCommandV1) { c.Purpose = "world" },
		func(c *StorageArtifactZIPValidationCommandV1) { c.ValidatorVersion = "latest" },
		func(c *StorageArtifactZIPValidationCommandV1) { c.Limits.MaxEntries++ },
		func(c *StorageArtifactZIPValidationCommandV1) { c.Limits.MaxTotalUncompressedBytes++ },
		func(c *StorageArtifactZIPValidationCommandV1) { c.Limits.MaxEntryUncompressedBytes++ },
		func(c *StorageArtifactZIPValidationCommandV1) { c.Limits.MaxCompressionRatio++ },
		func(c *StorageArtifactZIPValidationCommandV1) { c.Limits.MaxManifestBytes++ },
	}
	for i, mutate := range mutations {
		command := valid
		mutate(&command)
		if err := command.Validate(); err == nil {
			t.Fatalf("mutation %d validated: %#v", i, command)
		}
	}
}

func TestStorageArtifactZIPValidationRequestHasNoAmbientAuthority(t *testing.T) {
	command := validStorageArtifactZIPValidationCommandV1()
	wire, err := msgpack.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeStorageArtifactZIPValidationCommandV1(wire); err != nil {
		t.Fatalf("canonical request: %v", err)
	}

	base := map[string]any{
		"contract_version":    command.ContractVersion,
		"upload_id":           command.UploadID,
		"object_key":          command.ObjectKey,
		"expected_generation": command.ExpectedGeneration,
		"content_length":      command.ContentLength,
		"purpose":             command.Purpose,
		"validator_version":   command.ValidatorVersion,
		"limits":              command.Limits,
	}
	for _, forbidden := range []string{"bucket", "url", "headers", "range", "path", "provider"} {
		expanded := make(map[string]any, len(base)+1)
		for key, value := range base {
			expanded[key] = value
		}
		expanded[forbidden] = "ambient-authority"
		unknown, err := msgpack.Marshal(expanded)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := decodeStorageArtifactZIPValidationCommandV1(unknown); err == nil {
			t.Fatalf("request with forbidden field %q decoded", forbidden)
		}
	}

	nestedUnknown, err := msgpack.Marshal(map[string]any{
		"contract_version":    command.ContractVersion,
		"upload_id":           command.UploadID,
		"object_key":          command.ObjectKey,
		"expected_generation": command.ExpectedGeneration,
		"content_length":      command.ContentLength,
		"purpose":             command.Purpose,
		"validator_version":   command.ValidatorVersion,
		"limits": map[string]any{
			"max_entries":                  command.Limits.MaxEntries,
			"max_total_uncompressed_bytes": command.Limits.MaxTotalUncompressedBytes,
			"max_entry_uncompressed_bytes": command.Limits.MaxEntryUncompressedBytes,
			"max_compression_ratio":        command.Limits.MaxCompressionRatio,
			"max_manifest_bytes":           command.Limits.MaxManifestBytes,
			"provider":                     "s3",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeStorageArtifactZIPValidationCommandV1(nestedUnknown); err == nil {
		t.Fatal("request with unknown nested limits field decoded")
	}
}

func TestStorageArtifactZIPValidationFailsClosedForUnknownOrMismatchedResults(t *testing.T) {
	command := validStorageArtifactZIPValidationCommandV1()
	old := storageArtifactZIPValidateWire
	t.Cleanup(func() { storageArtifactZIPValidateWire = old })

	base := validStorageArtifactZIPValidationResultV1(command)
	mutations := []func(*StorageArtifactZIPValidationResultV1){
		func(r *StorageArtifactZIPValidationResultV1) { r.ContractVersion = "v2" },
		func(r *StorageArtifactZIPValidationResultV1) { r.UploadID = "other-upload" },
		func(r *StorageArtifactZIPValidationResultV1) { r.ObjectKey = "sessions/other.zip" },
		func(r *StorageArtifactZIPValidationResultV1) { r.Generation = "etag-2" },
		func(r *StorageArtifactZIPValidationResultV1) { r.Purpose = StorageArtifactPurposeBedrockResourceV1 },
		func(r *StorageArtifactZIPValidationResultV1) { r.ObservedContentLength++ },
		func(r *StorageArtifactZIPValidationResultV1) { r.ValidatorVersion = "latest" },
	}
	for i, mutate := range mutations {
		result := base
		mutate(&result)
		storageArtifactZIPValidateWire = func([]byte) ([]byte, uint32) {
			wire, err := msgpack.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			return wire, 0
		}
		if _, err := ValidateExactObjectArtifactZIP(command); err == nil {
			t.Fatalf("mismatched result mutation %d accepted: %#v", i, result)
		}
	}

	unknown, err := msgpack.Marshal(map[string]any{
		"contract_version":        base.ContractVersion,
		"upload_id":               base.UploadID,
		"object_key":              base.ObjectKey,
		"generation":              base.Generation,
		"purpose":                 base.Purpose,
		"valid":                   base.Valid,
		"observed_content_length": base.ObservedContentLength,
		"sha256":                  base.SHA256,
		"validator_version":       base.ValidatorVersion,
		"provider_etag":           "must-not-escape",
	})
	if err != nil {
		t.Fatal(err)
	}
	storageArtifactZIPValidateWire = func([]byte) ([]byte, uint32) { return unknown, 0 }
	if _, err := ValidateExactObjectArtifactZIP(command); err == nil {
		t.Fatal("response with unknown provider field accepted")
	}
}

func TestStorageArtifactZIPValidationResultBoundsAndManifestEvidence(t *testing.T) {
	command := validStorageArtifactZIPValidationCommandV1()
	valid := validStorageArtifactZIPValidationResultV1(command)
	mutations := []func(*StorageArtifactZIPValidationResultV1){
		func(r *StorageArtifactZIPValidationResultV1) { r.ObservedContentLength = 0 },
		func(r *StorageArtifactZIPValidationResultV1) { r.SHA256 = strings.Repeat("A", 64) },
		func(r *StorageArtifactZIPValidationResultV1) { r.SHA256 = "sha256:" + strings.Repeat("a", 64) },
		func(r *StorageArtifactZIPValidationResultV1) { r.Valid = false },
		func(r *StorageArtifactZIPValidationResultV1) { r.ErrorCode = "bad" },
		func(r *StorageArtifactZIPValidationResultV1) {
			r.Valid, r.ErrorCode = false, "BAD CODE"
		},
		func(r *StorageArtifactZIPValidationResultV1) {
			r.Valid, r.ErrorCode = false, strings.Repeat("a", 65)
		},
		func(r *StorageArtifactZIPValidationResultV1) { r.ManifestUUID = "1234" },
		func(r *StorageArtifactZIPValidationResultV1) {
			r.ManifestUUID = "12345678-1234-1234-1234-123456789abc"
			r.ManifestVersion = "1.0.0"
		},
	}
	for i, mutate := range mutations {
		result := valid
		mutate(&result)
		if err := result.Validate(); err == nil {
			t.Fatalf("invalid result mutation %d validated: %#v", i, result)
		}
	}

	bedrock := valid
	bedrock.Purpose = StorageArtifactPurposeBedrockBehaviorV1
	bedrock.ManifestUUID = "12345678-1234-1234-1234-123456789abc"
	bedrock.ManifestVersion = "1.2.3"
	if err := bedrock.Validate(); err != nil {
		t.Fatalf("valid Bedrock manifest evidence: %v", err)
	}
	bedrock.ManifestUUID = strings.ToUpper(bedrock.ManifestUUID)
	if err := bedrock.Validate(); err == nil {
		t.Fatal("uppercase non-canonical manifest UUID validated")
	}
	bedrock.ManifestUUID = "12345678-1234-1234-1234-123456789abc"
	bedrock.ManifestVersion = strings.Repeat("v", 65)
	if err := bedrock.Validate(); err == nil {
		t.Fatal("oversized manifest version validated")
	}
	bedrock.ManifestVersion = "1.2\n3"
	if err := bedrock.Validate(); err == nil {
		t.Fatal("control character in manifest version validated")
	}
}

func TestStorageArtifactZIPValidationNativeUnavailableAndABICodes(t *testing.T) {
	command := validStorageArtifactZIPValidationCommandV1()
	old := storageArtifactZIPValidateWire
	t.Cleanup(func() { storageArtifactZIPValidateWire = old })
	storageArtifactZIPValidateWire = func([]byte) ([]byte, uint32) { return nil, 99 }
	if _, err := ValidateExactObjectArtifactZIP(command); !errors.Is(err, pulp.ErrCapabilityUnavailable) {
		t.Fatalf("native capability error = %v", err)
	}

	if err := storageArtifactValidationCodeError(0); err != nil {
		t.Fatalf("code 0: %v", err)
	}
	for code := uint32(1); code <= 10; code++ {
		if err := storageArtifactValidationCodeError(code); err == nil {
			t.Fatalf("code %d succeeded", code)
		}
	}
	for _, code := range []uint32{11, 99} {
		if err := storageArtifactValidationCodeError(code); !errors.Is(err, pulp.ErrCapabilityUnavailable) {
			t.Fatalf("code %d = %v", code, err)
		}
	}
	if err := storageArtifactValidationCodeError(88); err == nil ||
		errors.Is(err, pulp.ErrCapabilityUnavailable) {
		t.Fatalf("unknown ABI code = %v", err)
	}
}
