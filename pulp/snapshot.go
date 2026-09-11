package pulp

import (
	"fmt"
	"sync"
	"unsafe"
)

// SnapshotFunc captures the cell's complete migratable state. The returned
// bytes are opaque to Pulp: the old and new versions of the cell own their
// schema and any required migration. A fused ABI-v2 cell returns one aggregate
// whose member partitions are assembled by its generated wrapper.
type SnapshotFunc func() ([]byte, error)

// RestoreFunc installs a snapshot captured from the previous live cell.
// Implementations should validate the complete payload before making state
// visible so a failed live update can be rolled back safely.
type RestoreFunc func(snapshot []byte) error

var snapshotState struct {
	sync.RWMutex
	snapshot SnapshotFunc
	restore  RestoreFunc
}

var (
	snapshotErrorMu sync.RWMutex
	snapshotError   []byte
	restoreErrorMu  sync.RWMutex
	restoreError    []byte
)

const maxSnapshotErrorBytes = 4096

// OnSnapshot registers the state capture hook used by pulp_snapshot.
func OnSnapshot(fn SnapshotFunc) {
	snapshotState.Lock()
	snapshotState.snapshot = fn
	snapshotState.Unlock()
}

// OnRestore registers the state installation hook used by pulp_restore.
func OnRestore(fn RestoreFunc) {
	snapshotState.Lock()
	snapshotState.restore = fn
	snapshotState.Unlock()
}

func boundedSnapshotError(err error) []byte {
	if err == nil {
		return nil
	}
	b := []byte(err.Error())
	if len(b) > maxSnapshotErrorBytes {
		b = append(b[:maxSnapshotErrorBytes-3:maxSnapshotErrorBytes-3], '.', '.', '.')
	}
	return b
}

func runSnapshot() (payload []byte, err error) {
	defer func() {
		if value := recover(); value != nil {
			payload = nil
			err = fmt.Errorf("snapshot panic: %v", value)
		}
	}()
	snapshotState.RLock()
	fn := snapshotState.snapshot
	snapshotState.RUnlock()
	if fn == nil {
		return nil, fmt.Errorf("snapshot hook is not registered")
	}
	payload, err = fn()
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), payload...), nil
}

func runRestore(payload []byte) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("restore panic: %v", value)
		}
	}()
	snapshotState.RLock()
	fn := snapshotState.restore
	snapshotState.RUnlock()
	if fn == nil {
		return fmt.Errorf("restore hook is not registered")
	}
	return fn(append([]byte(nil), payload...))
}

// pulp_snapshot writes a little-endian-compatible pair of uint32 values
// (response pointer, response length) at outPtr. The response is pinned in the
// same allocation table as pulp_alloc and must be released with pulp_free.
//
//go:wasmexport pulp_snapshot
func pulpSnapshot(outPtr uint32) (rc int32) {
	writeOutPtr(outPtr, 0)
	writeOutLen(outPtr+4, 0)
	payload, err := runSnapshot()
	snapshotErrorMu.Lock()
	snapshotError = boundedSnapshotError(err)
	snapshotErrorMu.Unlock()
	if err != nil {
		return 1
	}
	if len(payload) == 0 {
		return 0
	}
	ptr := pulpAlloc(uint32(len(payload)))
	if ptr == 0 {
		snapshotErrorMu.Lock()
		snapshotError = []byte("allocate snapshot response")
		snapshotErrorMu.Unlock()
		return 1
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len(payload)), payload)
	writeOutPtr(outPtr, ptr)
	writeOutLen(outPtr+4, uint32(len(payload)))
	return 0
}

//go:wasmexport pulp_restore
func pulpRestore(ptr, size uint32) (rc int32) {
	var payload []byte
	if size > 0 {
		if ptr == 0 {
			restoreErrorMu.Lock()
			restoreError = []byte("restore payload pointer is zero")
			restoreErrorMu.Unlock()
			return 1
		}
		payload = unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), size)
	}
	err := runRestore(payload)
	restoreErrorMu.Lock()
	restoreError = boundedSnapshotError(err)
	restoreErrorMu.Unlock()
	if err != nil {
		return 1
	}
	return 0
}

//go:wasmexport pulp_snapshot_error_ptr
func pulpSnapshotErrorPtr() uint32 {
	snapshotErrorMu.RLock()
	defer snapshotErrorMu.RUnlock()
	if len(snapshotError) == 0 {
		return 0
	}
	return uint32(uintptr(unsafe.Pointer(&snapshotError[0])))
}

//go:wasmexport pulp_snapshot_error_len
func pulpSnapshotErrorLen() uint32 {
	snapshotErrorMu.RLock()
	defer snapshotErrorMu.RUnlock()
	return uint32(len(snapshotError))
}

//go:wasmexport pulp_restore_error_ptr
func pulpRestoreErrorPtr() uint32 {
	restoreErrorMu.RLock()
	defer restoreErrorMu.RUnlock()
	if len(restoreError) == 0 {
		return 0
	}
	return uint32(uintptr(unsafe.Pointer(&restoreError[0])))
}

//go:wasmexport pulp_restore_error_len
func pulpRestoreErrorLen() uint32 {
	restoreErrorMu.RLock()
	defer restoreErrorMu.RUnlock()
	return uint32(len(restoreError))
}
