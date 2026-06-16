package pulp

// TODO: streaming read API — chunked fs_read with offset+length; needed for world archives >100MB. Blocked on ABI addition.

import (
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// FS groups host-import wrappers for the storage.fs capability.
// The cell must declare "storage.fs" in its manifest.
var FS = fsAPI{}

type fsAPI struct{}

//go:wasmimport pulp fs_read
func hostFSRead(pathPtr, pathLen, dataPtrOut, dataLenOut uint32) uint32

//go:wasmimport pulp fs_write
func hostFSWrite(pathPtr, pathLen, dataPtr, dataLen, reqPtr, reqLen uint32) uint32

//go:wasmimport pulp fs_delete
func hostFSDelete(pathPtr, pathLen uint32) uint32

//go:wasmimport pulp fs_list
func hostFSList(pathPtr, pathLen, dataPtrOut, dataLenOut uint32) uint32

//go:wasmimport pulp fs_stat
func hostFSStat(reqPtr, reqLen, dataPtrOut, dataLenOut uint32) uint32

//go:wasmimport pulp fs_rename
func hostFSRename(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp fs_remove_all
func hostFSRemoveAll(pathPtr, pathLen uint32) uint32

//go:wasmimport pulp fs_mkdir_all
func hostFSMkdirAll(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp fs_chmod
func hostFSChmod(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp fs_create_temp
func hostFSCreateTemp(reqPtr, reqLen, dataPtrOut, dataLenOut uint32) uint32

//go:wasmimport pulp fs_mkdir_temp
func hostFSMkdirTemp(reqPtr, reqLen, dataPtrOut, dataLenOut uint32) uint32

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
	case 99:
		return nil, ErrCapabilityUnavailable
	default:
		return nil, fmt.Errorf("fs_read host code %d", code)
	}
	if dataLen == 0 {
		return nil, nil
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(dataPtr))), dataLen)
	out := make([]byte, dataLen)
	copy(out, src)
	releaseHostAlloc(dataPtr, dataLen)
	return out, nil
}

// Write writes data to path with 0o644 permissions, truncating any existing
// file and creating parent directories as needed. All paths are relative to
// the cell's scoped storage root; absolute paths and .. traversal are
// rejected.
func (fsAPI) Write(path string, data []byte) error {
	return FS.WriteMode(path, data, 0o644)
}

// WriteMode is like Write but lets the caller specify the file mode.
func (fsAPI) WriteMode(path string, data []byte, perm os.FileMode) error {
	pathBytes := []byte(path)
	var dataPtr, dataLen uint32
	if len(data) > 0 {
		dataPtr = uint32(uintptr(unsafe.Pointer(&data[0])))
		dataLen = uint32(len(data))
	}
	reqBytes, err := msgpack.Marshal(struct {
		Mode uint32 `msgpack:"mode"`
	}{Mode: uint32(perm)})
	if err != nil {
		return fmt.Errorf("encode fs_write req: %w", err)
	}
	code := hostFSWrite(
		uint32(uintptr(unsafe.Pointer(&pathBytes[0]))),
		uint32(len(pathBytes)),
		dataPtr,
		dataLen,
		uint32(uintptr(unsafe.Pointer(&reqBytes[0]))),
		uint32(len(reqBytes)),
	)
	runtime.KeepAlive(pathBytes)
	runtime.KeepAlive(data)
	runtime.KeepAlive(reqBytes)
	switch code {
	case 0:
		return nil
	case 99:
		return ErrCapabilityUnavailable
	default:
		return fmt.Errorf("fs_write host code %d", code)
	}
}

// FileEntry represents a single directory entry returned by List.
type FileEntry struct {
	Name  string `msgpack:"name"`
	IsDir bool   `msgpack:"is_dir"`
}

// FileInfo is the metadata returned by Stat. ModTimeNs is unix nanoseconds;
// Mode carries the raw bits from os.FileMode.
type FileInfo struct {
	Name      string `msgpack:"name"`
	Size      int64  `msgpack:"size"`
	ModTimeNs int64  `msgpack:"mod_time_ns"`
	Mode      uint32 `msgpack:"mode"`
	IsDir     bool   `msgpack:"is_dir"`
}

// List returns the entries in the directory at path. Returns
// ErrNotFound when the directory does not exist.
func (fsAPI) List(path string) ([]FileEntry, error) {
	pathBytes := []byte(path)
	// An empty path means "list the scope root" — a legit call. Guard the
	// pointer so &pathBytes[0] doesn't panic (index out of range) on the empty
	// slice; the host treats len 0 as the root.
	var pathPtr uint32
	if len(pathBytes) > 0 {
		pathPtr = uint32(uintptr(unsafe.Pointer(&pathBytes[0])))
	}
	dataPtr := new(uint32)
	dataLen := new(uint32)
	code := hostFSList(
		pathPtr,
		uint32(len(pathBytes)),
		uint32(uintptr(unsafe.Pointer(dataPtr))),
		uint32(uintptr(unsafe.Pointer(dataLen))),
	)
	runtime.KeepAlive(pathBytes)
	runtime.KeepAlive(dataPtr)
	runtime.KeepAlive(dataLen)
	dPtr := *dataPtr
	dLen := *dataLen
	dataLen2 := dLen
	_ = dataLen2
	switch code {
	case 0:
		// ok
	case 6:
		return nil, ErrNotFound
	case 99:
		return nil, ErrCapabilityUnavailable
	default:
		return nil, fmt.Errorf("fs_list host code %d", code)
	}
	if dLen == 0 {
		return nil, nil
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(dPtr))), dLen)
	buf := make([]byte, dLen)
	copy(buf, src)
	releaseHostAlloc(dPtr, dLen)
	var entries []FileEntry
	if err := msgpack.Unmarshal(buf, &entries); err != nil {
		return nil, fmt.Errorf("decode fs_list: %w", err)
	}
	return entries, nil
}

// releaseHostAlloc tells the cell's allocator it can drop a buffer
// the host returned via writeResponse. Callers should invoke this
// immediately after copying the bytes into their own GC-owned slice.
// See pulpAlloc in lifecycle.go for why the map-backed allocator is
// necessary at all.
func releaseHostAlloc(ptr, size uint32) {
	pulpFree(ptr, size)
}

// ReleaseHostAlloc is the exported equivalent of releaseHostAlloc for
// use by sub-packages (entropy, s3, docker, stripe, …) that receive
// host-allocated buffers via respPtrOut/respLenOut parameters and must
// free them immediately after copying the bytes into a GC-owned slice.
func ReleaseHostAlloc(ptr, size uint32) {
	pulpFree(ptr, size)
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
	case 99:
		return ErrCapabilityUnavailable
	default:
		return fmt.Errorf("fs_delete host code %d", code)
	}
}

// Stat returns FileInfo for path. Returns ErrNotFound when the entry
// does not exist.
func (fsAPI) Stat(path string) (FileInfo, error) {
	reqBytes, err := msgpack.Marshal(struct {
		Path string `msgpack:"path"`
	}{Path: path})
	if err != nil {
		return FileInfo{}, fmt.Errorf("encode fs_stat req: %w", err)
	}
	var dataPtr, dataLen uint32
	code := hostFSStat(
		uint32(uintptr(unsafe.Pointer(&reqBytes[0]))),
		uint32(len(reqBytes)),
		uint32(uintptr(unsafe.Pointer(&dataPtr))),
		uint32(uintptr(unsafe.Pointer(&dataLen))),
	)
	runtime.KeepAlive(reqBytes)
	switch code {
	case 0:
		// ok
	case 6:
		return FileInfo{}, ErrNotFound
	case 99:
		return FileInfo{}, ErrCapabilityUnavailable
	default:
		return FileInfo{}, fmt.Errorf("fs_stat host code %d", code)
	}
	if dataLen == 0 {
		return FileInfo{}, nil
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(dataPtr))), dataLen)
	buf := make([]byte, dataLen)
	copy(buf, src)
	releaseHostAlloc(dataPtr, dataLen)
	var info FileInfo
	if err := msgpack.Unmarshal(buf, &info); err != nil {
		return FileInfo{}, fmt.Errorf("decode fs_stat: %w", err)
	}
	return info, nil
}

// Rename atomically renames oldPath to newPath. Both must resolve within
// the cell's scoped storage root.
func (fsAPI) Rename(oldPath, newPath string) error {
	reqBytes, err := msgpack.Marshal(struct {
		Old string `msgpack:"old"`
		New string `msgpack:"new"`
	}{Old: oldPath, New: newPath})
	if err != nil {
		return fmt.Errorf("encode fs_rename req: %w", err)
	}
	code := hostFSRename(
		uint32(uintptr(unsafe.Pointer(&reqBytes[0]))),
		uint32(len(reqBytes)),
	)
	runtime.KeepAlive(reqBytes)
	switch code {
	case 0:
		return nil
	case 6:
		return ErrNotFound
	case 99:
		return ErrCapabilityUnavailable
	default:
		return fmt.Errorf("fs_rename host code %d", code)
	}
}

// RemoveAll removes path and any children it contains. A missing path is
// not an error, matching os.RemoveAll.
func (fsAPI) RemoveAll(path string) error {
	pathBytes := []byte(path)
	code := hostFSRemoveAll(
		uint32(uintptr(unsafe.Pointer(&pathBytes[0]))),
		uint32(len(pathBytes)),
	)
	runtime.KeepAlive(pathBytes)
	switch code {
	case 0:
		return nil
	case 99:
		return ErrCapabilityUnavailable
	default:
		return fmt.Errorf("fs_remove_all host code %d", code)
	}
}

// MkdirAll creates path and any missing parents with the given permissions.
// A perm of 0 defaults to 0o755.
func (fsAPI) MkdirAll(path string, perm os.FileMode) error {
	reqBytes, err := msgpack.Marshal(struct {
		Path string `msgpack:"path"`
		Mode uint32 `msgpack:"mode"`
	}{Path: path, Mode: uint32(perm)})
	if err != nil {
		return fmt.Errorf("encode fs_mkdir_all req: %w", err)
	}
	code := hostFSMkdirAll(
		uint32(uintptr(unsafe.Pointer(&reqBytes[0]))),
		uint32(len(reqBytes)),
	)
	runtime.KeepAlive(reqBytes)
	switch code {
	case 0:
		return nil
	case 99:
		return ErrCapabilityUnavailable
	default:
		return fmt.Errorf("fs_mkdir_all host code %d", code)
	}
}

// Chmod changes the file mode of path. Only the permission bits (0o777) are
// applied; higher bits are stripped by the host. Returns ErrNotFound when
// path does not exist.
func (fsAPI) Chmod(path string, perm os.FileMode) error {
	reqBytes, err := msgpack.Marshal(struct {
		Path string `msgpack:"path"`
		Mode uint32 `msgpack:"mode"`
	}{Path: path, Mode: uint32(perm)})
	if err != nil {
		return fmt.Errorf("encode fs_chmod req: %w", err)
	}
	code := hostFSChmod(
		uint32(uintptr(unsafe.Pointer(&reqBytes[0]))),
		uint32(len(reqBytes)),
	)
	runtime.KeepAlive(reqBytes)
	switch code {
	case 0:
		return nil
	case 6:
		return ErrNotFound
	case 99:
		return ErrCapabilityUnavailable
	default:
		return fmt.Errorf("fs_chmod host code %d", code)
	}
}

// CreateTemp creates a new temporary file inside the cell's scoped root and
// returns its path relative to that root. If dir is empty, the file is placed
// in a default "tmp/" directory. pattern follows os.CreateTemp semantics: an
// asterisk is replaced with a random suffix, otherwise the suffix is appended.
// The file is closed by the host; callers should use Read/Write/Delete with
// the returned path.
func (fsAPI) CreateTemp(dir, pattern string) (string, error) {
	return fsTempCall(dir, pattern, hostFSCreateTemp, "fs_create_temp")
}

// MkdirTemp creates a new temporary directory inside the cell's scoped root
// and returns its path relative to that root. If dir is empty, the directory
// is placed in a default "tmp/" directory. pattern follows os.MkdirTemp
// semantics.
func (fsAPI) MkdirTemp(dir, pattern string) (string, error) {
	return fsTempCall(dir, pattern, hostFSMkdirTemp, "fs_mkdir_temp")
}

func fsTempCall(dir, pattern string, host func(uint32, uint32, uint32, uint32) uint32, name string) (string, error) {
	reqBytes, err := msgpack.Marshal(struct {
		Dir     string `msgpack:"dir"`
		Pattern string `msgpack:"pattern"`
	}{Dir: dir, Pattern: pattern})
	if err != nil {
		return "", fmt.Errorf("encode %s req: %w", name, err)
	}
	var dataPtr, dataLen uint32
	code := host(
		uint32(uintptr(unsafe.Pointer(&reqBytes[0]))),
		uint32(len(reqBytes)),
		uint32(uintptr(unsafe.Pointer(&dataPtr))),
		uint32(uintptr(unsafe.Pointer(&dataLen))),
	)
	runtime.KeepAlive(reqBytes)
	switch code {
	case 0:
		// ok
	case 6:
		return "", ErrNotFound
	case 99:
		return "", ErrCapabilityUnavailable
	default:
		return "", fmt.Errorf("%s host code %d", name, code)
	}
	if dataLen == 0 {
		return "", nil
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(dataPtr))), dataLen)
	buf := make([]byte, dataLen)
	copy(buf, src)
	releaseHostAlloc(dataPtr, dataLen)
	var resp struct {
		Path string `msgpack:"path"`
	}
	if err := msgpack.Unmarshal(buf, &resp); err != nil {
		return "", fmt.Errorf("decode %s: %w", name, err)
	}
	return resp.Path, nil
}
