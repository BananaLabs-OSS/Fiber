package effect

import (
	"bytes"
	"fmt"
	"mime"
	"strings"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

// StoragePublicUploadCapability is the deliberately narrow host grant for a
// browser/direct-client PUT to one owner-derived object. It is not storage.s3:
// it never gives a cell list, delete, bucket, prefix, or arbitrary-key access.
const StoragePublicUploadCapability = "storage.s3.public-upload.v1"

const (
	StorageExactObjectPresignPutImport  = "s3_exact_object_presign_put"
	StorageExactObjectValidatePutImport = "s3_exact_object_validate_put"
	// StorageExactObjectDeleteImport deletes exactly one owner-derived object
	// under its persisted ETag fence. It has no prefix, list, or bucket input.
	StorageExactObjectDeleteImport = "s3_exact_object_delete"

	// StorageExactObjectUploadMaxBytes is a hard host/guest ABI limit for one
	// public upload. Larger transfers need a separately designed multipart
	// contract rather than widening this direct-upload capability.
	StorageExactObjectUploadMaxBytes int64 = 64 << 20
	storageExactObjectUploadMaxTTL         = 15 * 60
)

// StorageObjectGenerationAbsent is the explicit create-only generation fence.
// Any other value is an exact ETag supplied by the state owner.
const StorageObjectGenerationAbsent = "absent"

// StorageExactObjectPresignPutCommand asks the host to sign exactly one PUT.
// ExactKey and UploadID are generated and persisted by the owning state cell;
// the captured host scope supplies the application identity, never this wire.
type StorageExactObjectPresignPutCommand struct {
	ExactKey           string `msgpack:"exact_key"`
	UploadID           string `msgpack:"upload_id"`
	ExpectedGeneration string `msgpack:"expected_generation"`
	ContentLength      int64  `msgpack:"content_length"`
	ContentType        string `msgpack:"content_type"`
	TTLSec             int64  `msgpack:"ttl_sec"`
}

// StorageExactObjectPresignPutResult returns a single constrained URL. The
// URL is intentionally not durable host state; replaying the same command may
// produce a new, equivalently constrained URL before its short expiry.
type StorageExactObjectPresignPutResult struct {
	ExactKey           string `msgpack:"exact_key"`
	UploadID           string `msgpack:"upload_id"`
	ExpectedGeneration string `msgpack:"expected_generation"`
	ContentLength      int64  `msgpack:"content_length"`
	ContentType        string `msgpack:"content_type"`
	URL                string `msgpack:"url"`
	ExpiresAtUnix      int64  `msgpack:"expires_at_unix"`
}

// StorageExactObjectValidatePutCommand verifies the exact object after a
// client PUT. It reads only metadata through HEAD; it never fetches a body.
type StorageExactObjectValidatePutCommand struct {
	ExactKey      string `msgpack:"exact_key"`
	UploadID      string `msgpack:"upload_id"`
	ContentLength int64  `msgpack:"content_length"`
	ContentType   string `msgpack:"content_type"`
}

// StorageExactObjectValidatePutResult is the durable post-upload inventory.
// Generation is the exact ETag observed during validation and must be stored
// by the owner before it finalizes its workflow.
type StorageExactObjectValidatePutResult struct {
	ExactKey      string `msgpack:"exact_key"`
	UploadID      string `msgpack:"upload_id"`
	Generation    string `msgpack:"generation"`
	ContentLength int64  `msgpack:"content_length"`
	ContentType   string `msgpack:"content_type"`
}

// StorageExactObjectDeleteCommand removes one exact object only when it still
// has the owner-recorded generation. The explicit absent fence is never valid
// for deletion: callers must first possess observed object inventory.
type StorageExactObjectDeleteCommand struct {
	ExactKey           string `msgpack:"exact_key"`
	ExpectedGeneration string `msgpack:"expected_generation"`
}

// StorageExactObjectDeleteResult is replay-stable completion evidence. A
// delete of an already-absent object has the same result as the original
// delete; generation changes fail instead of deleting newer content.
type StorageExactObjectDeleteResult struct {
	ExactKey           string `msgpack:"exact_key"`
	ExpectedGeneration string `msgpack:"expected_generation"`
}

func (c StorageExactObjectPresignPutCommand) Validate() error {
	if err := validateExactObjectKey("storage public upload exact key", c.ExactKey); err != nil {
		return err
	}
	if err := validateField("storage public upload id", c.UploadID); err != nil {
		return err
	}
	if err := validateStorageObjectGeneration(c.ExpectedGeneration); err != nil {
		return err
	}
	if err := validateStoragePublicUploadContent(c.ContentLength, c.ContentType); err != nil {
		return err
	}
	if c.TTLSec <= 0 || c.TTLSec > storageExactObjectUploadMaxTTL {
		return fmt.Errorf("storage public upload TTL must be between one and %d seconds", storageExactObjectUploadMaxTTL)
	}
	return nil
}

func (r StorageExactObjectPresignPutResult) Validate() error {
	if err := validateExactObjectKey("storage public upload result exact key", r.ExactKey); err != nil {
		return err
	}
	if err := validateField("storage public upload result id", r.UploadID); err != nil {
		return err
	}
	if err := validateStorageObjectGeneration(r.ExpectedGeneration); err != nil {
		return err
	}
	if err := validateStoragePublicUploadContent(r.ContentLength, r.ContentType); err != nil {
		return err
	}
	if strings.TrimSpace(r.URL) == "" || len(r.URL) > 8192 {
		return fmt.Errorf("storage public upload result URL is required")
	}
	if r.ExpiresAtUnix <= 0 {
		return fmt.Errorf("storage public upload result expiry is required")
	}
	return nil
}

func (c StorageExactObjectValidatePutCommand) Validate() error {
	if err := validateExactObjectKey("storage public upload validation exact key", c.ExactKey); err != nil {
		return err
	}
	if err := validateField("storage public upload validation id", c.UploadID); err != nil {
		return err
	}
	return validateStoragePublicUploadContent(c.ContentLength, c.ContentType)
}

func (r StorageExactObjectValidatePutResult) Validate() error {
	if err := validateExactObjectKey("storage public upload validation result exact key", r.ExactKey); err != nil {
		return err
	}
	if err := validateField("storage public upload validation result id", r.UploadID); err != nil {
		return err
	}
	if err := validateStorageObjectGeneration(r.Generation); err != nil || r.Generation == StorageObjectGenerationAbsent {
		return fmt.Errorf("storage public upload validation result generation is required")
	}
	return validateStoragePublicUploadContent(r.ContentLength, r.ContentType)
}

func (c StorageExactObjectDeleteCommand) Validate() error {
	if err := validateExactObjectKey("storage exact-object delete key", c.ExactKey); err != nil {
		return err
	}
	if err := validateStorageObjectGeneration(c.ExpectedGeneration); err != nil || c.ExpectedGeneration == StorageObjectGenerationAbsent {
		return fmt.Errorf("storage exact-object delete requires an observed generation")
	}
	return nil
}

func (r StorageExactObjectDeleteResult) Validate() error {
	return StorageExactObjectDeleteCommand{ExactKey: r.ExactKey, ExpectedGeneration: r.ExpectedGeneration}.Validate()
}

// PresignExactObjectPut calls the narrow host ABI. A calling cell must declare
// storage.s3.public-upload.v1; native tests replace the package-local seam.
func PresignExactObjectPut(command StorageExactObjectPresignPutCommand) (StorageExactObjectPresignPutResult, error) {
	if err := command.Validate(); err != nil {
		return StorageExactObjectPresignPutResult{}, fmt.Errorf("storage public upload: invalid presign command: %w", err)
	}
	request, err := msgpack.Marshal(command)
	if err != nil {
		return StorageExactObjectPresignPutResult{}, fmt.Errorf("storage public upload: marshal presign command: %w", err)
	}
	wire, code := storageExactObjectPresignPutWire(request)
	if err := storagePublicUploadCodeError(code); err != nil {
		return StorageExactObjectPresignPutResult{}, err
	}
	result, err := decodeStorageExactObjectPresignPutResult(wire)
	if err != nil {
		return StorageExactObjectPresignPutResult{}, err
	}
	if result.ExactKey != command.ExactKey || result.UploadID != command.UploadID || result.ExpectedGeneration != command.ExpectedGeneration || result.ContentLength != command.ContentLength || result.ContentType != command.ContentType {
		return StorageExactObjectPresignPutResult{}, fmt.Errorf("storage public upload: presign result does not match command")
	}
	return result, nil
}

// ValidateExactObjectPut calls the bounded post-upload HEAD ABI.
func ValidateExactObjectPut(command StorageExactObjectValidatePutCommand) (StorageExactObjectValidatePutResult, error) {
	if err := command.Validate(); err != nil {
		return StorageExactObjectValidatePutResult{}, fmt.Errorf("storage public upload: invalid validation command: %w", err)
	}
	request, err := msgpack.Marshal(command)
	if err != nil {
		return StorageExactObjectValidatePutResult{}, fmt.Errorf("storage public upload: marshal validation command: %w", err)
	}
	wire, code := storageExactObjectValidatePutWire(request)
	if err := storagePublicUploadCodeError(code); err != nil {
		return StorageExactObjectValidatePutResult{}, err
	}
	result, err := decodeStorageExactObjectValidatePutResult(wire)
	if err != nil {
		return StorageExactObjectValidatePutResult{}, err
	}
	if result.ExactKey != command.ExactKey || result.UploadID != command.UploadID || result.ContentLength != command.ContentLength || result.ContentType != command.ContentType {
		return StorageExactObjectValidatePutResult{}, fmt.Errorf("storage public upload: validation result does not match command")
	}
	return result, nil
}

// DeleteExactObject calls the exact-generation deletion ABI. It is safe to
// replay after a completed delete: absence is a successful terminal state,
// while a replacement object fails the ETag fence.
func DeleteExactObject(command StorageExactObjectDeleteCommand) (StorageExactObjectDeleteResult, error) {
	if err := command.Validate(); err != nil {
		return StorageExactObjectDeleteResult{}, fmt.Errorf("storage exact-object delete: invalid command: %w", err)
	}
	request, err := msgpack.Marshal(command)
	if err != nil {
		return StorageExactObjectDeleteResult{}, fmt.Errorf("storage exact-object delete: marshal command: %w", err)
	}
	wire, code := storageExactObjectDeleteWire(request)
	if err := storagePublicUploadCodeError(code); err != nil {
		return StorageExactObjectDeleteResult{}, err
	}
	result, err := decodeStorageExactObjectDeleteResult(wire)
	if err != nil {
		return StorageExactObjectDeleteResult{}, err
	}
	if result.ExactKey != command.ExactKey || result.ExpectedGeneration != command.ExpectedGeneration {
		return StorageExactObjectDeleteResult{}, fmt.Errorf("storage exact-object delete: result does not match command")
	}
	return result, nil
}

func validateStorageObjectGeneration(value string) error {
	if value == StorageObjectGenerationAbsent {
		return nil
	}
	if err := validateField("storage public upload expected generation", value); err != nil {
		return err
	}
	if len(value) > 256 || strings.ContainsAny(value, "\r\n\x00") {
		return fmt.Errorf("storage public upload expected generation is invalid")
	}
	return nil
}

func validateStoragePublicUploadContent(length int64, contentType string) error {
	if length <= 0 || length > StorageExactObjectUploadMaxBytes {
		return fmt.Errorf("storage public upload content length must be between one and %d bytes", StorageExactObjectUploadMaxBytes)
	}
	if len(contentType) == 0 || len(contentType) > 128 || strings.ContainsAny(contentType, "\r\n\x00") {
		return fmt.Errorf("storage public upload content type is invalid")
	}
	parsed, params, err := mime.ParseMediaType(contentType)
	if err != nil || len(params) != 0 || parsed != contentType || !strings.Contains(parsed, "/") {
		return fmt.Errorf("storage public upload content type must be one canonical media type without parameters")
	}
	return nil
}

func decodeStorageExactObjectPresignPutResult(raw []byte) (StorageExactObjectPresignPutResult, error) {
	var result StorageExactObjectPresignPutResult
	decoder := msgpack.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("storage public upload: decode presign result: %w", err)
	}
	if err := result.Validate(); err != nil {
		return result, fmt.Errorf("storage public upload: invalid presign result: %w", err)
	}
	return result, nil
}

func decodeStorageExactObjectValidatePutResult(raw []byte) (StorageExactObjectValidatePutResult, error) {
	var result StorageExactObjectValidatePutResult
	decoder := msgpack.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("storage public upload: decode validation result: %w", err)
	}
	if err := result.Validate(); err != nil {
		return result, fmt.Errorf("storage public upload: invalid validation result: %w", err)
	}
	return result, nil
}

func decodeStorageExactObjectDeleteResult(raw []byte) (StorageExactObjectDeleteResult, error) {
	var result StorageExactObjectDeleteResult
	decoder := msgpack.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("storage exact-object delete: decode result: %w", err)
	}
	if err := result.Validate(); err != nil {
		return result, fmt.Errorf("storage exact-object delete: invalid result: %w", err)
	}
	return result, nil
}

func storagePublicUploadCodeError(code uint32) error {
	switch code {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("storage public upload: empty request")
	case 2:
		return fmt.Errorf("storage public upload: request memory read failed")
	case 3:
		return fmt.Errorf("storage public upload: request decode failed")
	case 4:
		return fmt.Errorf("storage public upload: invalid request")
	case 5:
		return fmt.Errorf("storage public upload: provider operation failed")
	case 6:
		return fmt.Errorf("storage public upload: exact object not found")
	case 7:
		return fmt.Errorf("storage public upload: response encode failed")
	case 8:
		return fmt.Errorf("storage public upload: response allocation failed")
	case 9:
		return fmt.Errorf("storage public upload: response memory write failed")
	case 10:
		return fmt.Errorf("storage public upload: provider unavailable")
	case 11:
		return fmt.Errorf("storage exact-object delete: generation mismatch")
	case 99:
		return fmt.Errorf("storage public upload: %w", pulp.ErrCapabilityUnavailable)
	default:
		return fmt.Errorf("storage public upload: host code %d", code)
	}
}
