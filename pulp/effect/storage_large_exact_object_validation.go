package effect

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/vmihailenco/msgpack/v5"
)

// StorageLargeExactObjectValidationContractV1 is an additive protocol carried
// over StorageArtifactValidationCapability. The original small-object ZIP
// protocol remains unchanged.
const StorageLargeExactObjectValidationContractV1 = "storage.large-exact-object-validation.v1"

const (
	StorageLargeExactObjectValidationClaimVersionV1 = "storage.large-exact-object-validation.claim.v1"
	StorageObjectGenerationRefVersionV1             = "contracts.v1"

	StorageLargeExactObjectValidationMaxContentBytesV1 = int64(500 << 20)
	StorageLargeExactObjectValidationMaxEntriesV1      = uint32(200_000)
	StorageLargeExactObjectValidationMaxTotalBytesV1   = int64(4 << 30)
	StorageLargeExactObjectValidationMaxEntryBytesV1   = int64(1 << 30)
	StorageLargeExactObjectValidationMaxRatioV1        = uint32(1_000)
)

// StorageLargeExactObjectValidationLimitsV1 is a compiled safety profile. It
// bounds disk, central-directory memory, and declared decompression expansion;
// callers cannot widen it.
type StorageLargeExactObjectValidationLimitsV1 struct {
	MaxContentBytes           int64  `msgpack:"max_content_bytes"`
	MaxEntries                uint32 `msgpack:"max_entries"`
	MaxTotalUncompressedBytes int64  `msgpack:"max_total_uncompressed_bytes"`
	MaxEntryUncompressedBytes int64  `msgpack:"max_entry_uncompressed_bytes"`
	MaxCompressionRatio       uint32 `msgpack:"max_compression_ratio"`
}

func StorageLargeExactObjectValidationFixedLimitsV1() StorageLargeExactObjectValidationLimitsV1 {
	return StorageLargeExactObjectValidationLimitsV1{
		MaxContentBytes:           StorageLargeExactObjectValidationMaxContentBytesV1,
		MaxEntries:                StorageLargeExactObjectValidationMaxEntriesV1,
		MaxTotalUncompressedBytes: StorageLargeExactObjectValidationMaxTotalBytesV1,
		MaxEntryUncompressedBytes: StorageLargeExactObjectValidationMaxEntryBytesV1,
		MaxCompressionRatio:       StorageLargeExactObjectValidationMaxRatioV1,
	}
}

func (value StorageLargeExactObjectValidationLimitsV1) Validate() error {
	if value != StorageLargeExactObjectValidationFixedLimitsV1() {
		return fmt.Errorf("large exact-object validation limits do not match the compiled profile")
	}
	return nil
}

// StorageLargeExactObjectValidationClaimV1 is application-authored opaque
// authority. The host does not interpret it; it binds every result to the
// exact durable claim selected by the application.
type StorageLargeExactObjectValidationClaimV1 struct {
	Version string `msgpack:"version"`
	ID      string `msgpack:"id"`
	Digest  string `msgpack:"digest"`
}

func (value StorageLargeExactObjectValidationClaimV1) Validate() error {
	if value.Version != StorageLargeExactObjectValidationClaimVersionV1 {
		return fmt.Errorf("large exact-object validation claim version is invalid")
	}
	if err := validateField("large exact-object validation claim id", value.ID); err != nil {
		return err
	}
	if !storageArtifactSHA256V1.MatchString(value.Digest) {
		return fmt.Errorf("large exact-object validation claim digest is invalid")
	}
	return nil
}

// StorageObjectGenerationRefV1 is the terminal, provider-neutral object
// attestation shape shared with application contracts. StorageGeneration is
// carried separately because provider generations are opaque strings.
type StorageObjectGenerationRefV1 struct {
	Version    string `msgpack:"version"`
	Namespace  string `msgpack:"namespace"`
	Key        string `msgpack:"key"`
	Generation uint64 `msgpack:"generation"`
	SHA256     string `msgpack:"sha256"`
	SizeBytes  int64  `msgpack:"size_bytes"`
}

func (value StorageObjectGenerationRefV1) Validate() error {
	if value.Version != StorageObjectGenerationRefVersionV1 {
		return fmt.Errorf("object generation ref version is invalid")
	}
	if !validStorageArtifactBoundedTextV1(value.Namespace, 512) {
		return fmt.Errorf("object generation ref namespace is invalid")
	}
	if err := validateExactObjectKey("object generation ref key", value.Key); err != nil {
		return err
	}
	if value.Generation == 0 {
		return fmt.Errorf("object generation ref generation is required")
	}
	if !storageArtifactSHA256V1.MatchString(value.SHA256) {
		return fmt.Errorf("object generation ref sha256 is invalid")
	}
	if value.SizeBytes <= 0 || value.SizeBytes > StorageLargeExactObjectValidationMaxContentBytesV1 {
		return fmt.Errorf("object generation ref size is invalid")
	}
	return nil
}

// StorageLargeExactObjectValidationCommandV1 binds one opaque claim and
// validator policy to one exact object generation and expected digest.
type StorageLargeExactObjectValidationCommandV1 struct {
	ContractVersion    string                                    `msgpack:"contract_version"`
	Claim              StorageLargeExactObjectValidationClaimV1  `msgpack:"claim"`
	ObjectNamespace    string                                    `msgpack:"object_namespace"`
	ObjectKey          string                                    `msgpack:"object_key"`
	ExpectedGeneration string                                    `msgpack:"expected_generation"`
	ContentLength      int64                                     `msgpack:"content_length"`
	ExpectedSHA256     string                                    `msgpack:"expected_sha256"`
	ValidatorPolicy    string                                    `msgpack:"validator_policy"`
	Limits             StorageLargeExactObjectValidationLimitsV1 `msgpack:"limits"`
}

func (value StorageLargeExactObjectValidationCommandV1) Validate() error {
	if value.ContractVersion != StorageLargeExactObjectValidationContractV1 {
		return fmt.Errorf("large exact-object validation contract_version is invalid")
	}
	if err := value.Claim.Validate(); err != nil {
		return err
	}
	if !validStorageArtifactBoundedTextV1(value.ObjectNamespace, 512) {
		return fmt.Errorf("large exact-object validation namespace is invalid")
	}
	if err := validateExactObjectKey("large exact-object validation object key", value.ObjectKey); err != nil {
		return err
	}
	if !strings.HasSuffix(value.ObjectKey, ".zip") {
		return fmt.Errorf("large exact-object validation requires a ZIP object")
	}
	if err := validateStorageArtifactGenerationV1(value.ExpectedGeneration); err != nil {
		return err
	}
	if value.ContentLength <= 0 || value.ContentLength > StorageLargeExactObjectValidationMaxContentBytesV1 {
		return fmt.Errorf("large exact-object validation content length is invalid")
	}
	if !storageArtifactSHA256V1.MatchString(value.ExpectedSHA256) {
		return fmt.Errorf("large exact-object validation expected sha256 is invalid")
	}
	if !validStorageArtifactBoundedTextV1(value.ValidatorPolicy, 256) {
		return fmt.Errorf("large exact-object validation validator policy is invalid")
	}
	return value.Limits.Validate()
}

// StorageLargeExactObjectValidationResultV1 is terminal validation evidence.
// Object is present only for a valid, digest-matching archive.
type StorageLargeExactObjectValidationResultV1 struct {
	ContractVersion       string                                   `msgpack:"contract_version"`
	Claim                 StorageLargeExactObjectValidationClaimV1 `msgpack:"claim"`
	ObjectNamespace       string                                   `msgpack:"object_namespace"`
	ObjectKey             string                                   `msgpack:"object_key"`
	StorageGeneration     string                                   `msgpack:"storage_generation"`
	ObservedContentLength int64                                    `msgpack:"observed_content_length"`
	ExpectedSHA256        string                                   `msgpack:"expected_sha256"`
	ObservedSHA256        string                                   `msgpack:"observed_sha256"`
	ValidatorPolicy       string                                   `msgpack:"validator_policy"`
	Valid                 bool                                     `msgpack:"valid"`
	ErrorCode             string                                   `msgpack:"error_code,omitempty"`
	Object                *StorageObjectGenerationRefV1            `msgpack:"object,omitempty"`
}

func (value StorageLargeExactObjectValidationResultV1) Validate() error {
	if value.ContractVersion != StorageLargeExactObjectValidationContractV1 {
		return fmt.Errorf("large exact-object validation result contract_version is invalid")
	}
	if err := value.Claim.Validate(); err != nil {
		return err
	}
	if !validStorageArtifactBoundedTextV1(value.ObjectNamespace, 512) {
		return fmt.Errorf("large exact-object validation result namespace is invalid")
	}
	if err := validateExactObjectKey("large exact-object validation result object key", value.ObjectKey); err != nil {
		return err
	}
	if err := validateStorageArtifactGenerationV1(value.StorageGeneration); err != nil {
		return err
	}
	if value.ObservedContentLength <= 0 || value.ObservedContentLength > StorageLargeExactObjectValidationMaxContentBytesV1 {
		return fmt.Errorf("large exact-object validation result content length is invalid")
	}
	if !storageArtifactSHA256V1.MatchString(value.ExpectedSHA256) ||
		!storageArtifactSHA256V1.MatchString(value.ObservedSHA256) {
		return fmt.Errorf("large exact-object validation result sha256 is invalid")
	}
	if !validStorageArtifactBoundedTextV1(value.ValidatorPolicy, 256) {
		return fmt.Errorf("large exact-object validation result validator policy is invalid")
	}
	if value.Valid {
		if value.ErrorCode != "" || value.ObservedSHA256 != value.ExpectedSHA256 || value.Object == nil {
			return fmt.Errorf("valid large exact-object validation result is inconsistent")
		}
		if err := value.Object.Validate(); err != nil {
			return err
		}
		if value.Object.Namespace != value.ObjectNamespace || value.Object.Key != value.ObjectKey ||
			value.Object.SHA256 != value.ObservedSHA256 || value.Object.SizeBytes != value.ObservedContentLength {
			return fmt.Errorf("large exact-object validation object ref does not match evidence")
		}
	} else {
		if !storageArtifactErrorCodeV1.MatchString(value.ErrorCode) || value.Object != nil {
			return fmt.Errorf("invalid large exact-object validation result is inconsistent")
		}
	}
	return nil
}

// ValidateLargeExactObject invokes the additive protocol over the existing,
// narrow storage validation capability.
func ValidateLargeExactObject(command StorageLargeExactObjectValidationCommandV1) (StorageLargeExactObjectValidationResultV1, error) {
	if err := command.Validate(); err != nil {
		return StorageLargeExactObjectValidationResultV1{}, fmt.Errorf("large exact-object validation: invalid command: %w", err)
	}
	request, err := msgpack.Marshal(command)
	if err != nil {
		return StorageLargeExactObjectValidationResultV1{}, fmt.Errorf("large exact-object validation: marshal command: %w", err)
	}
	wire, code := storageArtifactZIPValidateWire(request)
	if err := storageArtifactValidationCodeError(code); err != nil {
		return StorageLargeExactObjectValidationResultV1{}, err
	}
	result, err := decodeStorageLargeExactObjectValidationResultV1(wire)
	if err != nil {
		return StorageLargeExactObjectValidationResultV1{}, err
	}
	if result.ContractVersion != command.ContractVersion ||
		result.Claim != command.Claim ||
		result.ObjectNamespace != command.ObjectNamespace ||
		result.ObjectKey != command.ObjectKey ||
		result.StorageGeneration != command.ExpectedGeneration ||
		result.ObservedContentLength != command.ContentLength ||
		result.ExpectedSHA256 != command.ExpectedSHA256 ||
		result.ValidatorPolicy != command.ValidatorPolicy {
		return StorageLargeExactObjectValidationResultV1{}, fmt.Errorf("large exact-object validation: result does not match exact claim and object")
	}
	return result, nil
}

func decodeStorageLargeExactObjectValidationCommandV1(raw []byte) (StorageLargeExactObjectValidationCommandV1, error) {
	var command StorageLargeExactObjectValidationCommandV1
	reader := bytes.NewReader(raw)
	decoder := msgpack.NewDecoder(reader)
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&command); err != nil {
		return command, fmt.Errorf("large exact-object validation: decode command: %w", err)
	}
	if reader.Len() != 0 {
		return command, fmt.Errorf("large exact-object validation: trailing command data")
	}
	if err := command.Validate(); err != nil {
		return command, fmt.Errorf("large exact-object validation: invalid command: %w", err)
	}
	return command, nil
}

func decodeStorageLargeExactObjectValidationResultV1(raw []byte) (StorageLargeExactObjectValidationResultV1, error) {
	var result StorageLargeExactObjectValidationResultV1
	reader := bytes.NewReader(raw)
	decoder := msgpack.NewDecoder(reader)
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("large exact-object validation: decode result: %w", err)
	}
	if reader.Len() != 0 {
		return result, fmt.Errorf("large exact-object validation: trailing result data")
	}
	if err := result.Validate(); err != nil {
		return result, fmt.Errorf("large exact-object validation: invalid result: %w", err)
	}
	return result, nil
}
