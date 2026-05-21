// Package s3 is the cell-side wrapper for the storage.s3 capability
// provided by Pulp-ext-s3 (AWS SDK v2 host-side). Cell code calls
// these methods to read and write objects in a Cloudflare R2 (or any
// S3-compatible) bucket without touching host imports directly.
//
//	import (
//		"github.com/BananaLabs-OSS/Fiber/pulp/s3"
//	)
//
//	if err := s3.Put("worlds/abc.tar", data); err != nil { ... }
//	url, err := s3.Presign("worlds/abc.tar", 15 * time.Minute)
//
// The cell's manifest must declare:
//
//	capabilities = ["storage.s3"]
//
// and the host binary must link Pulp-ext-s3 via blank import.
package s3

import (
	"errors"
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

// ErrS3AccessDenied is returned when the host's credentials lack
// permission for the requested operation (S3 AccessDenied). Mapped
// from host code 11.
var ErrS3AccessDenied = errors.New("pulp/s3: access denied")

//go:wasmimport pulp s3_put
func hostS3Put(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp s3_put_sized
func hostS3PutSized(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp s3_put_multipart_init
func hostS3PutMultipartInit(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_put_multipart_part
func hostS3PutMultipartPart(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_put_multipart_complete
func hostS3PutMultipartComplete(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp s3_put_multipart_abort
func hostS3PutMultipartAbort(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp s3_presign
func hostS3Presign(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_presign_put
func hostS3PresignPut(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_head
func hostS3Head(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_get
func hostS3Get(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_copy
func hostS3Copy(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp s3_delete
func hostS3Delete(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp s3_list
func hostS3List(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

type putRequest struct {
	Key  string `msgpack:"key"`
	Body []byte `msgpack:"body"`
}

type putSizedRequest struct {
	Key           string `msgpack:"key"`
	Body          []byte `msgpack:"body"`
	ContentLength int64  `msgpack:"content_length"`
	ContentType   string `msgpack:"content_type"`
}

type multipartInitRequest struct {
	Key         string `msgpack:"key"`
	ContentType string `msgpack:"content_type"`
}

type multipartInitResponse struct {
	UploadID string `msgpack:"upload_id"`
}

type multipartPartRequest struct {
	Key        string `msgpack:"key"`
	UploadID   string `msgpack:"upload_id"`
	PartNumber int32  `msgpack:"part_number"`
	Data       []byte `msgpack:"data"`
}

type multipartPartResponse struct {
	ETag string `msgpack:"etag"`
}

type multipartPart struct {
	PartNumber int32  `msgpack:"part_number"`
	ETag       string `msgpack:"etag"`
}

type multipartCompleteRequest struct {
	Key      string          `msgpack:"key"`
	UploadID string          `msgpack:"upload_id"`
	Parts    []multipartPart `msgpack:"parts"`
}

type multipartAbortRequest struct {
	Key      string `msgpack:"key"`
	UploadID string `msgpack:"upload_id"`
}

type presignRequest struct {
	Key    string `msgpack:"key"`
	TTLSec int64  `msgpack:"ttl_sec"`
}

type presignResponse struct {
	URL string `msgpack:"url"`
}

type headRequest struct {
	Key string `msgpack:"key"`
}

// HeadResult is the decoded response of Head — file size plus last-
// modified time. Both are zero when the object does not exist (and
// Head returns an error in that case).
type HeadResult struct {
	Size             int64 `msgpack:"size"`
	LastModifiedUnix int64 `msgpack:"last_modified_unix"`
}

type getRequest struct {
	Key string `msgpack:"key"`
}

// GetObjectResponse is the decoded result of GetObject — the whole
// object body plus headers reported by the host (content type, length,
// etag).
type GetObjectResponse struct {
	Body          []byte `msgpack:"body"`
	ContentType   string `msgpack:"content_type"`
	ContentLength int64  `msgpack:"content_length"`
	ETag          string `msgpack:"etag"`
}

type copyRequest struct {
	SrcKey string `msgpack:"src_key"`
	DstKey string `msgpack:"dst_key"`
}

type deleteRequest struct {
	Key string `msgpack:"key"`
}

type listRequest struct {
	Prefix            string `msgpack:"prefix"`
	ContinuationToken string `msgpack:"continuation_token"`
	MaxKeys           int32  `msgpack:"max_keys"`
}

// ListEntry is one object in a List page — the key plus size and
// last-modified (both zero when the server didn't return them).
type ListEntry struct {
	Key              string `msgpack:"key"`
	Size             int64  `msgpack:"size"`
	LastModifiedUnix int64  `msgpack:"last_modified_unix"`
}

type listResponse struct {
	Entries               []ListEntry `msgpack:"entries"`
	NextContinuationToken string      `msgpack:"next_continuation_token"`
	IsTruncated           bool        `msgpack:"is_truncated"`
}

// ListPage is one page of results returned by List. When IsTruncated
// is true, pass NextContinuationToken back into List to fetch the next
// page.
type ListPage struct {
	Entries               []ListEntry
	NextContinuationToken string
	IsTruncated           bool
}

// Put uploads body as the content at key in the configured bucket.
// Overwrites existing objects.
func Put(key string, body []byte) error {
	data, err := msgpack.Marshal(putRequest{Key: key, Body: body})
	if err != nil {
		return fmt.Errorf("encode put: %w", err)
	}
	code := hostS3Put(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	return codeToError("s3_put", code)
}

// PutSized uploads body to key with ContentLength set explicitly on
// the underlying PutObjectInput — the SDK then issues a known-length
// PUT rather than an aws-chunked streaming one. Mirrors r2.UploadSized.
// contentType may be empty.
func PutSized(key string, body []byte, contentType string) error {
	data, err := msgpack.Marshal(putSizedRequest{
		Key:           key,
		Body:          body,
		ContentLength: int64(len(body)),
		ContentType:   contentType,
	})
	if err != nil {
		return fmt.Errorf("encode put_sized: %w", err)
	}
	code := hostS3PutSized(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	return codeToError("s3_put_sized", code)
}

// DefaultMultipartChunkSize is the chunk size PutMultipart uses when the
// caller passes 0. 8 MB balances: (a) S3/R2's 5 MB minimum part size,
// (b) msgpack+ABI copy overhead per call, (c) host-side decode buffer
// bound. Under this, a 5 GB world archive sends in ~625 calls.
const DefaultMultipartChunkSize = 8 * 1024 * 1024

// MinMultipartChunkSize is the S3/R2 minimum part size (except the last
// part). Chunks smaller than this for non-final parts are rejected by
// the service.
const MinMultipartChunkSize = 5 * 1024 * 1024

// PutMultipart uploads body to key using S3 multipart, splitting into
// chunks of chunkSize (0 = DefaultMultipartChunkSize). Prefer this over
// Put for objects larger than ~16 MB — the whole body still lives in
// cell memory (body is []byte), but only one chunk at a time crosses
// the ABI, so host-side msgpack decode + S3 SDK buffering stay at
// chunkSize, not len(body). contentType may be empty.
//
// On any failure the in-progress multipart upload is aborted so R2
// does not hold orphaned parts.
func PutMultipart(key, contentType string, body []byte, chunkSize int) error {
	if chunkSize <= 0 {
		chunkSize = DefaultMultipartChunkSize
	}
	if len(body) == 0 {
		// Empty bodies don't round-trip through multipart (min 1 part).
		// Fall back to a plain Put.
		return Put(key, body)
	}
	// If the whole body is smaller than the minimum part size, S3
	// would reject a multipart with a single undersized part. Use a
	// regular Put — small bodies don't need streaming anyway.
	if len(body) < MinMultipartChunkSize {
		return Put(key, body)
	}

	uploadID, err := putMultipartInit(key, contentType)
	if err != nil {
		return err
	}

	parts := make([]multipartPart, 0, (len(body)+chunkSize-1)/chunkSize)
	for i, partNum := 0, int32(1); i < len(body); partNum++ {
		end := i + chunkSize
		if end > len(body) {
			end = len(body)
		}
		// S3 forbids parts < 5 MB except the last. If we would emit
		// a non-final part that's too small (caller passed a tiny
		// chunkSize), merge it into the last chunk we emit.
		if end-i < MinMultipartChunkSize && end != len(body) {
			// Re-chunking at this granularity isn't worth it — just
			// abort and fall back to plain Put.
			_ = putMultipartAbort(key, uploadID)
			return Put(key, body)
		}
		etag, err := putMultipartPart(key, uploadID, partNum, body[i:end])
		if err != nil {
			_ = putMultipartAbort(key, uploadID)
			return err
		}
		parts = append(parts, multipartPart{PartNumber: partNum, ETag: etag})
		i = end
	}

	if err := putMultipartComplete(key, uploadID, parts); err != nil {
		_ = putMultipartAbort(key, uploadID)
		return err
	}
	return nil
}

func putMultipartInit(key, contentType string) (string, error) {
	data, err := msgpack.Marshal(multipartInitRequest{Key: key, ContentType: contentType})
	if err != nil {
		return "", fmt.Errorf("encode multipart_init: %w", err)
	}
	var respPtr, respLen uint32
	code := hostS3PutMultipartInit(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("s3_put_multipart_init", code); err != nil {
		return "", err
	}
	if respLen == 0 {
		return "", fmt.Errorf("s3_put_multipart_init: empty response")
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp multipartInitResponse
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return "", fmt.Errorf("decode multipart_init: %w", err)
	}
	return resp.UploadID, nil
}

func putMultipartPart(key, uploadID string, partNumber int32, chunk []byte) (string, error) {
	req := multipartPartRequest{
		Key:        key,
		UploadID:   uploadID,
		PartNumber: partNumber,
		Data:       chunk,
	}
	data, err := msgpack.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("encode multipart_part: %w", err)
	}
	var respPtr, respLen uint32
	code := hostS3PutMultipartPart(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("s3_put_multipart_part", code); err != nil {
		return "", err
	}
	if respLen == 0 {
		return "", fmt.Errorf("s3_put_multipart_part: empty response")
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp multipartPartResponse
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return "", fmt.Errorf("decode multipart_part: %w", err)
	}
	return resp.ETag, nil
}

func putMultipartComplete(key, uploadID string, parts []multipartPart) error {
	data, err := msgpack.Marshal(multipartCompleteRequest{
		Key:      key,
		UploadID: uploadID,
		Parts:    parts,
	})
	if err != nil {
		return fmt.Errorf("encode multipart_complete: %w", err)
	}
	code := hostS3PutMultipartComplete(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
	)
	runtime.KeepAlive(data)
	return codeToError("s3_put_multipart_complete", code)
}

func putMultipartAbort(key, uploadID string) error {
	data, err := msgpack.Marshal(multipartAbortRequest{
		Key:      key,
		UploadID: uploadID,
	})
	if err != nil {
		return fmt.Errorf("encode multipart_abort: %w", err)
	}
	code := hostS3PutMultipartAbort(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
	)
	runtime.KeepAlive(data)
	return codeToError("s3_put_multipart_abort", code)
}

// Presign returns a pre-signed GET URL for key, valid for ttl.
// Typical use: give clients a direct download link without proxying.
func Presign(key string, ttl time.Duration) (string, error) {
	data, err := msgpack.Marshal(presignRequest{Key: key, TTLSec: int64(ttl / time.Second)})
	if err != nil {
		return "", fmt.Errorf("encode presign: %w", err)
	}
	var respPtr, respLen uint32
	code := hostS3Presign(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("s3_presign", code); err != nil {
		return "", err
	}
	if respLen == 0 {
		return "", fmt.Errorf("s3_presign: empty response")
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp presignResponse
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return "", fmt.Errorf("decode presign: %w", err)
	}
	return resp.URL, nil
}

// PresignPut returns a pre-signed PUT URL for key, valid for ttl.
// Use this to let clients upload directly to R2 without streaming
// bytes through the cell — avoids buffering large files in the
// cell's WASM memory.
func PresignPut(key string, ttl time.Duration) (string, error) {
	data, err := msgpack.Marshal(presignRequest{Key: key, TTLSec: int64(ttl / time.Second)})
	if err != nil {
		return "", fmt.Errorf("encode presign_put: %w", err)
	}
	var respPtr, respLen uint32
	code := hostS3PresignPut(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("s3_presign_put", code); err != nil {
		return "", err
	}
	if respLen == 0 {
		return "", fmt.Errorf("s3_presign_put: empty response")
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp presignResponse
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return "", fmt.Errorf("decode presign_put: %w", err)
	}
	return resp.URL, nil
}

// Head returns size + last-modified for key. Returns an error if the
// object does not exist.
func Head(key string) (HeadResult, error) {
	data, err := msgpack.Marshal(headRequest{Key: key})
	if err != nil {
		return HeadResult{}, fmt.Errorf("encode head: %w", err)
	}
	var respPtr, respLen uint32
	code := hostS3Head(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("s3_head", code); err != nil {
		return HeadResult{}, err
	}
	if respLen == 0 {
		return HeadResult{}, nil
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp HeadResult
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return HeadResult{}, fmt.Errorf("decode head: %w", err)
	}
	return resp, nil
}

// GetObject fetches the whole body of key plus reported content-type,
// content-length, and etag. Returns pulp.ErrNotFound if the object does
// not exist and ErrS3AccessDenied if the host lacks permission.
//
// Intended for small objects. For large downloads prefer Presign — the
// body is held entirely in both host and cell memory.
func GetObject(key string) (GetObjectResponse, error) {
	data, err := msgpack.Marshal(getRequest{Key: key})
	if err != nil {
		return GetObjectResponse{}, fmt.Errorf("encode get: %w", err)
	}
	var respPtr, respLen uint32
	code := hostS3Get(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("s3_get", code); err != nil {
		return GetObjectResponse{}, err
	}
	if respLen == 0 {
		return GetObjectResponse{}, nil
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp GetObjectResponse
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return GetObjectResponse{}, fmt.Errorf("decode get: %w", err)
	}
	return resp, nil
}

// Copy duplicates the object at srcKey to dstKey within the bucket.
func Copy(srcKey, dstKey string) error {
	data, err := msgpack.Marshal(copyRequest{SrcKey: srcKey, DstKey: dstKey})
	if err != nil {
		return fmt.Errorf("encode copy: %w", err)
	}
	code := hostS3Copy(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	return codeToError("s3_copy", code)
}

// Delete removes the object at key. Missing keys do NOT return an
// error — S3 / R2 delete is idempotent.
func Delete(key string) error {
	data, err := msgpack.Marshal(deleteRequest{Key: key})
	if err != nil {
		return fmt.Errorf("encode delete: %w", err)
	}
	code := hostS3Delete(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	return codeToError("s3_delete", code)
}

// List returns one page of objects in the configured bucket whose key
// starts with prefix. Pass an empty continuationToken for the first
// page; when the returned page has IsTruncated == true, pass its
// NextContinuationToken back in to fetch the next page. maxKeys caps
// the page size (<=0 leaves the default to the host; S3 caps at 1000).
//
// Typical use: orphan scans. Walk the bucket in pages, diff against
// the set of keys recorded in the DB, and delete the leftovers.
func List(prefix, continuationToken string, maxKeys int32) (ListPage, error) {
	data, err := msgpack.Marshal(listRequest{
		Prefix:            prefix,
		ContinuationToken: continuationToken,
		MaxKeys:           maxKeys,
	})
	if err != nil {
		return ListPage{}, fmt.Errorf("encode list: %w", err)
	}
	var respPtr, respLen uint32
	code := hostS3List(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("s3_list", code); err != nil {
		return ListPage{}, err
	}
	if respLen == 0 {
		return ListPage{}, nil
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp listResponse
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return ListPage{}, fmt.Errorf("decode list: %w", err)
	}
	return ListPage{
		Entries:               resp.Entries,
		NextContinuationToken: resp.NextContinuationToken,
		IsTruncated:           resp.IsTruncated,
	}, nil
}

// ListAll walks every page under prefix and returns every entry. Use
// for small prefixes only — each entry is held in memory. For large
// scans prefer paging with List.
func ListAll(prefix string) ([]ListEntry, error) {
	var out []ListEntry
	token := ""
	for {
		page, err := List(prefix, token, 1000)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Entries...)
		if !page.IsTruncated || page.NextContinuationToken == "" {
			return out, nil
		}
		token = page.NextContinuationToken
	}
}

// codeToError maps Pulp-ext-s3 host error codes to Go errors.
// 99 = capability not declared → pulp.ErrCapabilityUnavailable
// 10 = host not configured (missing S3_ACCESS_KEY_ID / etc)
// 6  = object not found (NoSuchKey / NotFound) → pulp.ErrNotFound
// 11 = access denied → ErrS3AccessDenied
// 4  = S3 API error (network, auth, other)
// Others are treated as generic protocol errors.
func codeToError(op string, code uint32) error {
	switch code {
	case 0:
		return nil
	case 99:
		return fmt.Errorf("%s: %w", op, pulp.ErrCapabilityUnavailable)
	case 10:
		return fmt.Errorf("%s: host missing S3 credentials", op)
	case 6:
		return fmt.Errorf("%s: %w", op, pulp.ErrNotFound)
	case 11:
		return fmt.Errorf("%s: %w", op, ErrS3AccessDenied)
	case 4:
		return fmt.Errorf("%s: s3 api error", op)
	default:
		return fmt.Errorf("%s: host code %d", op, code)
	}
}
