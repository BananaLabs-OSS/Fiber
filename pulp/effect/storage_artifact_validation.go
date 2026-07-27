package effect

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

// StorageArtifactValidationCapability is the narrow host grant for validating
// one owner-authored exact ZIP object. It exposes no bucket, prefix, URL,
// headers, ranges, filesystem paths, or provider selection.
const StorageArtifactValidationCapability = "storage.s3.artifact-validation.v1"

const (
	StorageArtifactValidationContractV1           = StorageArtifactValidationCapability
	StorageExactObjectArtifactZIPValidateImport   = "s3_exact_object_validate_artifact_zip"
	StorageArtifactZIPValidatorVersionV1          = "zip-artifact-validator.v1"
	StorageArtifactZIPValidationMaxContentBytesV1 = int64(50 << 20)

	StorageArtifactZIPValidationMaxEntriesV1                = uint32(4096)
	StorageArtifactZIPValidationMaxTotalUncompressedBytesV1 = int64(256 << 20)
	StorageArtifactZIPValidationMaxEntryUncompressedBytesV1 = int64(128 << 20)
	StorageArtifactZIPValidationMaxCompressionRatioV1       = uint32(100)
	StorageArtifactZIPValidationMaxManifestBytesV1          = int64(1 << 20)
)

type StorageArtifactPurposeV1 string

const (
	StorageArtifactPurposeDatapackV1        StorageArtifactPurposeV1 = "datapack"
	StorageArtifactPurposeBedrockResourceV1 StorageArtifactPurposeV1 = "bedrock_resource"
	StorageArtifactPurposeBedrockBehaviorV1 StorageArtifactPurposeV1 = "bedrock_behavior"
)

// StorageArtifactZIPValidationLimitsV1 is repeated on the request so a durable
// owner command records the exact validator policy it selected. Validate
// admits only the compiled constants: a guest cannot weaken or widen them.
type StorageArtifactZIPValidationLimitsV1 struct {
	MaxEntries                uint32 `msgpack:"max_entries"`
	MaxTotalUncompressedBytes int64  `msgpack:"max_total_uncompressed_bytes"`
	MaxEntryUncompressedBytes int64  `msgpack:"max_entry_uncompressed_bytes"`
	MaxCompressionRatio       uint32 `msgpack:"max_compression_ratio"`
	MaxManifestBytes          int64  `msgpack:"max_manifest_bytes"`
}

func StorageArtifactZIPValidationFixedLimitsV1() StorageArtifactZIPValidationLimitsV1 {
	return StorageArtifactZIPValidationLimitsV1{
		MaxEntries:                StorageArtifactZIPValidationMaxEntriesV1,
		MaxTotalUncompressedBytes: StorageArtifactZIPValidationMaxTotalUncompressedBytesV1,
		MaxEntryUncompressedBytes: StorageArtifactZIPValidationMaxEntryUncompressedBytesV1,
		MaxCompressionRatio:       StorageArtifactZIPValidationMaxCompressionRatioV1,
		MaxManifestBytes:          StorageArtifactZIPValidationMaxManifestBytesV1,
	}
}

// StorageArtifactZIPValidationCommandV1 requests validation of one exact,
// generation-fenced object. ObjectKey and UploadID are generated and persisted
// by the state owner; application scope and storage credentials stay host-side.
type StorageArtifactZIPValidationCommandV1 struct {
	ContractVersion    string                               `msgpack:"contract_version"`
	UploadID           string                               `msgpack:"upload_id"`
	ObjectKey          string                               `msgpack:"object_key"`
	ExpectedGeneration string                               `msgpack:"expected_generation"`
	ContentLength      int64                                `msgpack:"content_length"`
	Purpose            StorageArtifactPurposeV1             `msgpack:"purpose"`
	ValidatorVersion   string                               `msgpack:"validator_version"`
	Limits             StorageArtifactZIPValidationLimitsV1 `msgpack:"limits"`
}

// StorageArtifactZIPValidationResultV1 is bounded validation evidence for the
// exact command. ErrorCode is a stable machine code, never provider text.
type StorageArtifactZIPValidationResultV1 struct {
	ContractVersion       string                   `msgpack:"contract_version"`
	UploadID              string                   `msgpack:"upload_id"`
	ObjectKey             string                   `msgpack:"object_key"`
	Generation            string                   `msgpack:"generation"`
	Purpose               StorageArtifactPurposeV1 `msgpack:"purpose"`
	Valid                 bool                     `msgpack:"valid"`
	ObservedContentLength int64                    `msgpack:"observed_content_length"`
	SHA256                string                   `msgpack:"sha256"`
	ValidatorVersion      string                   `msgpack:"validator_version"`
	ErrorCode             string                   `msgpack:"error_code,omitempty"`
	ManifestUUID          string                   `msgpack:"manifest_uuid,omitempty"`
	ManifestVersion       string                   `msgpack:"manifest_version,omitempty"`
}

func (value StorageArtifactZIPValidationLimitsV1) Validate() error {
	if value != StorageArtifactZIPValidationFixedLimitsV1() {
		return fmt.Errorf("storage artifact validation limits must match validator version %q", StorageArtifactZIPValidatorVersionV1)
	}
	return nil
}

func (value StorageArtifactZIPValidationCommandV1) Validate() error {
	if value.ContractVersion != StorageArtifactValidationContractV1 {
		return fmt.Errorf("storage artifact validation contract_version is invalid")
	}
	if err := validateField("storage artifact validation upload_id", value.UploadID); err != nil {
		return err
	}
	if err := validateExactObjectKey("storage artifact validation object_key", value.ObjectKey); err != nil {
		return err
	}
	if !strings.HasSuffix(value.ObjectKey, ".zip") {
		return fmt.Errorf("storage artifact validation object_key must name a ZIP object")
	}
	if err := validateStorageArtifactGenerationV1(value.ExpectedGeneration); err != nil {
		return err
	}
	if value.ContentLength <= 0 || value.ContentLength > StorageArtifactZIPValidationMaxContentBytesV1 {
		return fmt.Errorf("storage artifact validation content_length must be between one and %d bytes", StorageArtifactZIPValidationMaxContentBytesV1)
	}
	if !validStorageArtifactPurposeV1(value.Purpose) {
		return fmt.Errorf("storage artifact validation purpose is not allowlisted")
	}
	if value.ValidatorVersion != StorageArtifactZIPValidatorVersionV1 {
		return fmt.Errorf("storage artifact validation validator_version is invalid")
	}
	return value.Limits.Validate()
}

func (value StorageArtifactZIPValidationResultV1) Validate() error {
	if value.ContractVersion != StorageArtifactValidationContractV1 {
		return fmt.Errorf("storage artifact validation result contract_version is invalid")
	}
	if err := validateField("storage artifact validation result upload_id", value.UploadID); err != nil {
		return err
	}
	if err := validateExactObjectKey("storage artifact validation result object_key", value.ObjectKey); err != nil {
		return err
	}
	if !strings.HasSuffix(value.ObjectKey, ".zip") {
		return fmt.Errorf("storage artifact validation result object_key must name a ZIP object")
	}
	if err := validateStorageArtifactGenerationV1(value.Generation); err != nil {
		return err
	}
	if !validStorageArtifactPurposeV1(value.Purpose) {
		return fmt.Errorf("storage artifact validation result purpose is not allowlisted")
	}
	if value.ObservedContentLength <= 0 || value.ObservedContentLength > StorageArtifactZIPValidationMaxContentBytesV1 {
		return fmt.Errorf("storage artifact validation result observed_content_length is invalid")
	}
	if !storageArtifactSHA256V1.MatchString(value.SHA256) {
		return fmt.Errorf("storage artifact validation result sha256 is invalid")
	}
	if value.ValidatorVersion != StorageArtifactZIPValidatorVersionV1 {
		return fmt.Errorf("storage artifact validation result validator_version is invalid")
	}
	if value.Valid {
		if value.ErrorCode != "" {
			return fmt.Errorf("valid storage artifact validation result cannot contain error_code")
		}
	} else if !storageArtifactErrorCodeV1.MatchString(value.ErrorCode) {
		return fmt.Errorf("invalid storage artifact validation result requires bounded error_code")
	}
	if (value.ManifestUUID == "") != (value.ManifestVersion == "") {
		return fmt.Errorf("storage artifact validation manifest UUID and version are required together")
	}
	if value.ManifestUUID != "" {
		if value.Purpose == StorageArtifactPurposeDatapackV1 || !value.Valid {
			return fmt.Errorf("storage artifact validation manifest evidence is not allowed")
		}
		if !storageArtifactManifestUUIDV1.MatchString(value.ManifestUUID) ||
			!validStorageArtifactBoundedTextV1(value.ManifestVersion, 64) {
			return fmt.Errorf("storage artifact validation manifest evidence is invalid")
		}
	}
	return nil
}

// ValidateExactObjectArtifactZIP invokes the one-operation host capability.
// The response must remain bound to every owner-controlled identity and fence.
func ValidateExactObjectArtifactZIP(command StorageArtifactZIPValidationCommandV1) (StorageArtifactZIPValidationResultV1, error) {
	if err := command.Validate(); err != nil {
		return StorageArtifactZIPValidationResultV1{}, fmt.Errorf("storage artifact validation: invalid command: %w", err)
	}
	request, err := msgpack.Marshal(command)
	if err != nil {
		return StorageArtifactZIPValidationResultV1{}, fmt.Errorf("storage artifact validation: marshal command: %w", err)
	}
	wire, code := storageArtifactZIPValidateWire(request)
	if err := storageArtifactValidationCodeError(code); err != nil {
		return StorageArtifactZIPValidationResultV1{}, err
	}
	result, err := decodeStorageArtifactZIPValidationResultV1(wire)
	if err != nil {
		return StorageArtifactZIPValidationResultV1{}, err
	}
	if result.ContractVersion != command.ContractVersion ||
		result.UploadID != command.UploadID ||
		result.ObjectKey != command.ObjectKey ||
		result.Generation != command.ExpectedGeneration ||
		result.Purpose != command.Purpose ||
		result.ObservedContentLength != command.ContentLength ||
		result.ValidatorVersion != command.ValidatorVersion {
		return StorageArtifactZIPValidationResultV1{}, fmt.Errorf("storage artifact validation: result does not match exact command")
	}
	return result, nil
}

func decodeStorageArtifactZIPValidationCommandV1(raw []byte) (StorageArtifactZIPValidationCommandV1, error) {
	var command StorageArtifactZIPValidationCommandV1
	decoder := msgpack.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&command); err != nil {
		return command, fmt.Errorf("storage artifact validation: decode command: %w", err)
	}
	if err := command.Validate(); err != nil {
		return command, fmt.Errorf("storage artifact validation: invalid command: %w", err)
	}
	return command, nil
}

func decodeStorageArtifactZIPValidationResultV1(raw []byte) (StorageArtifactZIPValidationResultV1, error) {
	var result StorageArtifactZIPValidationResultV1
	decoder := msgpack.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("storage artifact validation: decode result: %w", err)
	}
	if err := result.Validate(); err != nil {
		return result, fmt.Errorf("storage artifact validation: invalid result: %w", err)
	}
	return result, nil
}

func validateStorageArtifactGenerationV1(value string) error {
	if value == StorageObjectGenerationAbsent {
		return fmt.Errorf("storage artifact validation requires an observed object generation")
	}
	if err := validateField("storage artifact validation generation", value); err != nil {
		return err
	}
	return nil
}

func validStorageArtifactPurposeV1(value StorageArtifactPurposeV1) bool {
	switch value {
	case StorageArtifactPurposeDatapackV1,
		StorageArtifactPurposeBedrockResourceV1,
		StorageArtifactPurposeBedrockBehaviorV1:
		return true
	default:
		return false
	}
}

func validStorageArtifactBoundedTextV1(value string, limit int) bool {
	return value != "" && len(value) <= limit && strings.TrimSpace(value) == value &&
		strings.IndexFunc(value, unicode.IsControl) < 0
}

func storageArtifactValidationCodeError(code uint32) error {
	switch code {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("storage artifact validation: empty request")
	case 2:
		return fmt.Errorf("storage artifact validation: request memory read failed")
	case 3:
		return fmt.Errorf("storage artifact validation: request decode failed")
	case 4:
		return fmt.Errorf("storage artifact validation: invalid request")
	case 5:
		return fmt.Errorf("storage artifact validation: provider operation failed")
	case 6:
		return fmt.Errorf("storage artifact validation: exact object not found")
	case 7:
		return fmt.Errorf("storage artifact validation: object generation mismatch")
	case 8:
		return fmt.Errorf("storage artifact validation: response encode failed")
	case 9:
		return fmt.Errorf("storage artifact validation: response allocation failed")
	case 10:
		return fmt.Errorf("storage artifact validation: response memory write failed")
	case 11, 99:
		return fmt.Errorf("storage artifact validation: %w", pulp.ErrCapabilityUnavailable)
	default:
		return fmt.Errorf("storage artifact validation: host code %d", code)
	}
}

var storageArtifactSHA256V1 = regexp.MustCompile(`^[0-9a-f]{64}$`)
var storageArtifactErrorCodeV1 = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)
var storageArtifactManifestUUIDV1 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
