// Package s3 is the plugin-side wrapper for the storage.s3 capability
// provided by Pulp-ext-s3 (AWS SDK v2 host-side). Plugin code calls
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
// The plugin's manifest must declare:
//
//	capabilities = ["storage.s3"]
//
// and the host binary must link Pulp-ext-s3 via blank import.
package s3

import (
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

//go:wasmimport pulp s3_put
func hostS3Put(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp s3_presign
func hostS3Presign(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_presign_put
func hostS3PresignPut(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_head
func hostS3Head(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_copy
func hostS3Copy(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp s3_delete
func hostS3Delete(reqPtr, reqLen uint32) uint32

type putRequest struct {
	Key  string `msgpack:"key"`
	Body []byte `msgpack:"body"`
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

type copyRequest struct {
	SrcKey string `msgpack:"src_key"`
	DstKey string `msgpack:"dst_key"`
}

type deleteRequest struct {
	Key string `msgpack:"key"`
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
// bytes through the plugin — avoids buffering large files in the
// plugin's WASM memory.
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

// codeToError maps Pulp-ext-s3 host error codes to Go errors.
// 99 = capability not declared (plugin's manifest is missing storage.s3)
// 10 = host not configured (missing S3_ACCESS_KEY_ID / etc)
// 4  = S3 API error (network, auth, not found)
// Others are treated as generic protocol errors.
func codeToError(op string, code uint32) error {
	switch code {
	case 0:
		return nil
	case 99:
		return fmt.Errorf("%s: capability storage.s3 not declared in manifest", op)
	case 10:
		return fmt.Errorf("%s: host missing S3 credentials", op)
	case 4:
		return fmt.Errorf("%s: s3 api error", op)
	default:
		return fmt.Errorf("%s: host code %d", op, code)
	}
}
