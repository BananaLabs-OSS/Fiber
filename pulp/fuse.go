//go:build wasip1

package pulp

// Cell-side client for the storage.fuse capability (Pulp-ext-fuse). The cell
// declares a policy-enforcing virtual drive (FUSE on Linux) with Mount, then
// observes what the drive does through "fuse.audit" / "fuse.denied" step events
// (decode with DecodeFuseEvent). The cell must declare "storage.fuse" in its
// manifest.
//
// Design note: per-op file I/O is served NATIVELY by the host core at full
// speed — the cell never sees individual reads/writes. Only coarse operations
// (mount/unmount) and policy DECISIONS (the audit/denied events) cross this
// boundary.

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// FUSE groups the host-import wrappers for the storage.fuse capability.
var FUSE = fuseAPI{}

type fuseAPI struct{}

// ErrFUSEUnavailable is returned when the host stubbed the capability — the cell
// did not declare "storage.fuse" in its manifest capabilities list.
var ErrFUSEUnavailable = fmt.Errorf("pulp: storage.fuse unavailable (declare it in cell manifest)")

// FuseAccess is the access level for a path prefix in a mount policy.
type FuseAccess int

const (
	// FuseNone denies all access to the prefix.
	FuseNone FuseAccess = 0
	// FuseRead allows read-only access.
	FuseRead FuseAccess = 1
	// FuseReadWrite allows read and write access.
	FuseReadWrite FuseAccess = 2
)

// FuseRule maps a path prefix (relative to the mount root) to an access level.
// Longest-prefix match wins; unmatched paths default to None (denied).
type FuseRule struct {
	Prefix string     `msgpack:"prefix"`
	Access FuseAccess `msgpack:"access"`
}

// FuseMountSpec declares a policy-enforcing virtual drive: mount Backing at
// Mountpoint, enforcing Rules on every operation.
type FuseMountSpec struct {
	Mountpoint string     `msgpack:"mountpoint"`
	Backing    string     `msgpack:"backing"`
	Rules      []FuseRule `msgpack:"rules"`
}

// FuseEvent is the payload of a "fuse.audit" / "fuse.denied" step event: the
// outcome of one policy decision the host core made while serving a native op.
type FuseEvent struct {
	MountID uint32 `msgpack:"mount_id"`
	Op      string `msgpack:"op"`
	Path    string `msgpack:"path"`
	Allowed bool   `msgpack:"allowed"`
}

const fuseFirstMountID = uint32(100)

//go:wasmimport pulp fuse_mount
func hostFuseMount(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp fuse_unmount
func hostFuseUnmount(mountID uint32) uint32

// Mount declares a policy-enforcing virtual drive and returns its mount id. On
// a non-Linux host (or any mount failure) it returns a non-nil error; the cell
// keeps running and simply has no drive.
func (fuseAPI) Mount(spec FuseMountSpec) (uint32, error) {
	req, err := msgpack.Marshal(spec)
	if err != nil {
		return 0, fmt.Errorf("encode fuse_mount req: %w", err)
	}
	code := hostFuseMount(
		uint32(uintptr(unsafe.Pointer(&req[0]))),
		uint32(len(req)),
	)
	runtime.KeepAlive(req)
	if code >= fuseFirstMountID {
		return code, nil
	}
	if code == 99 {
		return 0, ErrFUSEUnavailable
	}
	return 0, fmt.Errorf("fuse_mount host code %d", code)
}

// Unmount tears down a previously mounted virtual drive by id.
func (fuseAPI) Unmount(id uint32) error {
	code := hostFuseUnmount(id)
	if code == 0 {
		return nil
	}
	if code == 99 {
		return ErrFUSEUnavailable
	}
	return fmt.Errorf("fuse_unmount host code %d", code)
}

// DecodeFuseEvent decodes a "fuse.audit" or "fuse.denied" step event payload.
func DecodeFuseEvent(payload []byte) (FuseEvent, error) {
	var e FuseEvent
	err := msgpack.Unmarshal(payload, &e)
	return e, err
}
