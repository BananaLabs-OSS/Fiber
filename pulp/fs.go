package pulp

import (
	"fmt"
	"runtime"
	"unsafe"
)

// FS groups host-import wrappers for the storage.fs capability.
// The plugin must declare "storage.fs" in its manifest.
var FS = fsAPI{}

type fsAPI struct{}

//go:wasmimport pulp fs_read
func hostFSRead(pathPtr, pathLen, dataPtrOut, dataLenOut uint32) uint32

//go:wasmimport pulp fs_write
func hostFSWrite(pathPtr, pathLen, dataPtr, dataLen uint32) uint32

//go:wasmimport pulp fs_delete
func hostFSDelete(pathPtr, pathLen uint32) uint32

// ErrNotFound is the sentinel returned by Read and Delete when the
// target path does not exist.
var ErrNotFound = fmt.Errorf("not found")

// Read returns the file's bytes. Returns ErrNotFound when the file
// does not exist.
func (fsAPI) Read(path string) ([]byte, error) {
	pathBytes := []byte(path)
	var dataPtr, dataLen uint32
	code := hostFSRead(
		uint32(uintptr(unsafe.Pointer(&pathBytes[0]))),
		uint32(len(pathBytes)),
		uint32(uintptr(unsafe.Pointer(&dataPtr))),
		uint32(uintptr(unsafe.Pointer(&dataLen))),
	)
	runtime.KeepAlive(pathBytes)
	switch code {
	case 0:
		// ok
	case 6:
		return nil, ErrNotFound
	default:
		return nil, fmt.Errorf("fs_read host code %d", code)
	}
	if dataLen == 0 {
		return nil, nil
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(dataPtr))), dataLen)
	out := make([]byte, dataLen)
	copy(out, src)
	return out, nil
}

// Write writes data to path, truncating any existing file and creating
// parent directories as needed. All paths are relative to the plugin's
// scoped storage root; absolute paths and .. traversal are rejected.
func (fsAPI) Write(path string, data []byte) error {
	pathBytes := []byte(path)
	var dataPtr, dataLen uint32
	if len(data) > 0 {
		dataPtr = uint32(uintptr(unsafe.Pointer(&data[0])))
		dataLen = uint32(len(data))
	}
	code := hostFSWrite(
		uint32(uintptr(unsafe.Pointer(&pathBytes[0]))),
		uint32(len(pathBytes)),
		dataPtr,
		dataLen,
	)
	runtime.KeepAlive(pathBytes)
	runtime.KeepAlive(data)
	if code != 0 {
		return fmt.Errorf("fs_write host code %d", code)
	}
	return nil
}

// Delete removes the file at path. Returns ErrNotFound if the file
// is missing.
func (fsAPI) Delete(path string) error {
	pathBytes := []byte(path)
	code := hostFSDelete(
		uint32(uintptr(unsafe.Pointer(&pathBytes[0]))),
		uint32(len(pathBytes)),
	)
	runtime.KeepAlive(pathBytes)
	switch code {
	case 0:
		return nil
	case 6:
		return ErrNotFound
	default:
		return fmt.Errorf("fs_delete host code %d", code)
	}
}
