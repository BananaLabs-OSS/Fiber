//go:build wasip1

package effect

import (
	"runtime"
	"unsafe"

	"github.com/BananaLabs-OSS/Fiber/pulp"
)

//go:wasmimport pulp s3_exact_object_presign_put
func hostStorageExactObjectPresignPut(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_exact_object_validate_put
func hostStorageExactObjectValidatePut(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_exact_object_delete
func hostStorageExactObjectDelete(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

var storageExactObjectPresignPutWire = hostStorageExactObjectPresignPutWire
var storageExactObjectValidatePutWire = hostStorageExactObjectValidatePutWire
var storageExactObjectDeleteWire = hostStorageExactObjectDeleteWire

func hostStorageExactObjectPresignPutWire(request []byte) ([]byte, uint32) {
	return hostStorageExactObjectUploadWire(request, hostStorageExactObjectPresignPut)
}

func hostStorageExactObjectValidatePutWire(request []byte) ([]byte, uint32) {
	return hostStorageExactObjectUploadWire(request, hostStorageExactObjectValidatePut)
}

func hostStorageExactObjectDeleteWire(request []byte) ([]byte, uint32) {
	return hostStorageExactObjectUploadWire(request, hostStorageExactObjectDelete)
}

func hostStorageExactObjectUploadWire(request []byte, host func(uint32, uint32, uint32, uint32) uint32) ([]byte, uint32) {
	var responsePtr, responseLen uint32
	code := host(
		uint32(uintptr(unsafe.Pointer(&request[0]))), uint32(len(request)),
		uint32(uintptr(unsafe.Pointer(&responsePtr))), uint32(uintptr(unsafe.Pointer(&responseLen))),
	)
	runtime.KeepAlive(request)
	if code != 0 || responsePtr == 0 || responseLen == 0 {
		return nil, code
	}
	response := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(responsePtr))), responseLen)
	wire := append([]byte(nil), response...)
	pulp.ReleaseHostAlloc(responsePtr, responseLen)
	return wire, 0
}
