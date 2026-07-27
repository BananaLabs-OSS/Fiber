package effect

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

// StorageExactObjectDownloadReferenceCapability grants one generation-fenced
// public download reference. Bucket, prefix, endpoint, headers, and provider
// selection stay entirely host-owned.
const StorageExactObjectDownloadReferenceCapability = "storage.s3.exact-object-download-reference.v1"

const (
	StorageExactObjectDownloadReferenceContractV1 = "storage.exact-object-download-reference.v1"
	StorageExactObjectDownloadReferenceImport     = "s3_exact_object_download_reference"
)

// StorageExactObjectDownloadReferenceCommand is owner-persisted input for one
// exact object. ExpiresAtUnix is assigned by the owner and echoed by the host,
// keeping successful replay evidence stable; it is never derived from retry
// time inside a host extension.
type StorageExactObjectDownloadReferenceCommand struct {
	ContractVersion string `msgpack:"contract_version"`
	ObjectKey       string `msgpack:"object_key"`
	ExpectedETag    string `msgpack:"expected_etag"`
	ExpiresAtUnix   int64  `msgpack:"expires_at_unix"`
}

type StorageExactObjectDownloadReferenceResult struct {
	ContractVersion string `msgpack:"contract_version"`
	ObjectKey       string `msgpack:"object_key"`
	ExpectedETag    string `msgpack:"expected_etag"`
	PublicURL       string `msgpack:"public_url"`
	ExpiresAtUnix   int64  `msgpack:"expires_at_unix"`
}

func (value StorageExactObjectDownloadReferenceCommand) Validate() error {
	if value.ContractVersion != StorageExactObjectDownloadReferenceContractV1 {
		return fmt.Errorf("storage exact-object download reference contract_version is invalid")
	}
	if err := validateExactObjectKey("storage exact-object download reference key", value.ObjectKey); err != nil {
		return err
	}
	if err := validateStorageObjectGeneration(value.ExpectedETag); err != nil || value.ExpectedETag == StorageObjectGenerationAbsent {
		return fmt.Errorf("storage exact-object download reference requires an observed ETag")
	}
	if value.ExpiresAtUnix <= 0 {
		return fmt.Errorf("storage exact-object download reference expires_at_unix is required")
	}
	return nil
}

func (value StorageExactObjectDownloadReferenceResult) Validate() error {
	if err := (StorageExactObjectDownloadReferenceCommand{
		ContractVersion: value.ContractVersion, ObjectKey: value.ObjectKey,
		ExpectedETag: value.ExpectedETag, ExpiresAtUnix: value.ExpiresAtUnix,
	}).Validate(); err != nil {
		return err
	}
	if len(value.PublicURL) == 0 || len(value.PublicURL) > 8192 || strings.ContainsAny(value.PublicURL, "\r\n\x00") {
		return fmt.Errorf("storage exact-object download reference public_url is invalid")
	}
	parsed, err := url.Parse(value.PublicURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("storage exact-object download reference public_url must be a canonical HTTPS reference")
	}
	return nil
}

// DownloadReferenceExactObject invokes one narrow host operation. The host
// validates the current object generation before returning a configured stable
// provider URL; callers cannot turn this into an arbitrary presigner.
func DownloadReferenceExactObject(command StorageExactObjectDownloadReferenceCommand) (StorageExactObjectDownloadReferenceResult, error) {
	if err := command.Validate(); err != nil {
		return StorageExactObjectDownloadReferenceResult{}, fmt.Errorf("storage exact-object download reference: invalid command: %w", err)
	}
	request, err := msgpack.Marshal(command)
	if err != nil {
		return StorageExactObjectDownloadReferenceResult{}, fmt.Errorf("storage exact-object download reference: marshal command: %w", err)
	}
	wire, code := storageExactObjectDownloadReferenceWire(request)
	if err := storageExactObjectDownloadReferenceCodeError(code); err != nil {
		return StorageExactObjectDownloadReferenceResult{}, err
	}
	result, err := decodeStorageExactObjectDownloadReferenceResult(wire)
	if err != nil {
		return StorageExactObjectDownloadReferenceResult{}, err
	}
	if result.ContractVersion != command.ContractVersion || result.ObjectKey != command.ObjectKey || result.ExpectedETag != command.ExpectedETag || result.ExpiresAtUnix != command.ExpiresAtUnix {
		return StorageExactObjectDownloadReferenceResult{}, fmt.Errorf("storage exact-object download reference: result does not match command")
	}
	return result, nil
}

func decodeStorageExactObjectDownloadReferenceResult(raw []byte) (StorageExactObjectDownloadReferenceResult, error) {
	var result StorageExactObjectDownloadReferenceResult
	decoder := msgpack.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("storage exact-object download reference: decode result: %w", err)
	}
	if err := result.Validate(); err != nil {
		return result, fmt.Errorf("storage exact-object download reference: invalid result: %w", err)
	}
	return result, nil
}

func storageExactObjectDownloadReferenceCodeError(code uint32) error {
	switch code {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("storage exact-object download reference: empty request")
	case 2:
		return fmt.Errorf("storage exact-object download reference: request memory read failed")
	case 3:
		return fmt.Errorf("storage exact-object download reference: request decode failed")
	case 4:
		return fmt.Errorf("storage exact-object download reference: invalid request")
	case 5:
		return fmt.Errorf("storage exact-object download reference: provider operation failed")
	case 6:
		return fmt.Errorf("storage exact-object download reference: exact object not found")
	case 7:
		return fmt.Errorf("storage exact-object download reference: object generation mismatch")
	case 8:
		return fmt.Errorf("storage exact-object download reference: response encode failed")
	case 9:
		return fmt.Errorf("storage exact-object download reference: response allocation failed")
	case 10:
		return fmt.Errorf("storage exact-object download reference: response memory write failed")
	case 11, 99:
		return fmt.Errorf("storage exact-object download reference: %w", pulp.ErrCapabilityUnavailable)
	default:
		return fmt.Errorf("storage exact-object download reference: host code %d", code)
	}
}
