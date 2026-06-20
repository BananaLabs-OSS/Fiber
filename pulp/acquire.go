package pulp

// Cell-side client for the tool.acquire capability (Pulp-ext-acquire): ask the
// host to resolve an external CLI binary to an absolute path — by one of three
// sources (already-installed on PATH, the vendor's official installer, or a
// manual path). The cell is sandboxed and can't run installers or probe the
// host filesystem, so it delegates here. The cell must declare "tool.acquire"
// in its manifest.
//
// This only RESOLVES (returns a path). Any app-specific placement (e.g. copying
// the binary into a runtime dir) is the caller's job — keeping the capability
// reusable by any cell without inheriting one app's conventions.

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// Acquire groups the host-import wrappers for the tool.acquire capability.
var Acquire = acquireAPI{}

type acquireAPI struct{}

//go:wasmimport pulp tool_acquire
func hostToolAcquire(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

// AcquireResult is the resolved outcome. Path is the absolute binary path when
// Ok. Status is "resolved" (found on PATH / manual), "installed" (official
// installer ran), "notfound", "unsupported", or "failed"; Message is detail.
type AcquireResult struct {
	Ok      bool
	Status  string
	Path    string
	Message string
}

// Tool resolves a CLI binary by name via the given source:
//   - "installed": binary already on PATH (or at expectPath).
//   - "official":  run installCmd (the vendor's official installer) then re-resolve.
//   - "manual":    verify the explicit path.
//
// expectPath is an optional fallback location to check when the binary isn't on
// PATH after an install (e.g. "~/.local/bin/claude"). installCmd/path are only
// used by the matching source.
func (acquireAPI) Tool(name, source, installCmd, path, expectPath string) (AcquireResult, error) {
	req, err := msgpack.Marshal(struct {
		Name       string `msgpack:"name"`
		Source     string `msgpack:"source"`
		InstallCmd string `msgpack:"install_cmd"`
		Path       string `msgpack:"path"`
		ExpectPath string `msgpack:"expect_path"`
	}{Name: name, Source: source, InstallCmd: installCmd, Path: path, ExpectPath: expectPath})
	if err != nil {
		return AcquireResult{}, err
	}
	var respPtr, respLen uint32
	code := hostToolAcquire(
		uint32(uintptr(unsafe.Pointer(&req[0]))), uint32(len(req)),
		uint32(uintptr(unsafe.Pointer(&respPtr))), uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(req)
	if code == 99 {
		return AcquireResult{}, ErrCapabilityUnavailable
	}
	if code != 0 {
		return AcquireResult{}, fmt.Errorf("tool_acquire host code %d", code)
	}
	if respLen == 0 {
		return AcquireResult{}, nil
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	buf := make([]byte, respLen)
	copy(buf, src)
	releaseHostAlloc(respPtr, respLen)
	var resp struct {
		Ok      bool   `msgpack:"ok"`
		Status  string `msgpack:"status"`
		Path    string `msgpack:"path"`
		Message string `msgpack:"message"`
	}
	if err := msgpack.Unmarshal(buf, &resp); err != nil {
		return AcquireResult{}, err
	}
	return AcquireResult{Ok: resp.Ok, Status: resp.Status, Path: resp.Path, Message: resp.Message}, nil
}
