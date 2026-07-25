package pulp

// Cell-side client for the toolchain.install capability (Pulp-ext-toolchain):
// ask the host to download + extract a language toolchain into its bundled
// runtime dir, and check whether a toolchain is already present there. The cell
// is sandboxed and can't write the host runtime, so it delegates here. The cell
// must declare "toolchain.install" in its manifest.

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// Toolchain groups the host-import wrappers for the toolchain.install capability.
var Toolchain = toolchainAPI{}

type toolchainAPI struct{}

// Install asks the host to download + extract the toolchain for lang (e.g.
// "go", "git") into its bundled runtime dir, picking the pinned URL for the
// host's own OS/arch. It is idempotent — an already-present toolchain returns
// ok=true, status="present". status is "present", "failed", or "unsupported"
// (unknown lang or no pin for the host platform); message is the binary path on
// success or an error string on failure.
func (toolchainAPI) Install(lang string) (ok bool, status string, msg string, err error) {
	req, err := msgpack.Marshal(struct {
		Lang string `msgpack:"lang"`
	}{Lang: lang})
	if err != nil {
		return false, "", "", err
	}
	var respPtr, respLen uint32
	code := hostToolchainInstall(
		uint32(uintptr(unsafe.Pointer(&req[0]))), uint32(len(req)),
		uint32(uintptr(unsafe.Pointer(&respPtr))), uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(req)
	if code == 99 {
		return false, "", "", ErrCapabilityUnavailable
	}
	if code != 0 {
		return false, "", "", fmt.Errorf("toolchain_install host code %d", code)
	}
	if respLen == 0 {
		return false, "", "", nil
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	buf := make([]byte, respLen)
	copy(buf, src)
	releaseHostAlloc(respPtr, respLen)
	var resp struct {
		Ok      bool   `msgpack:"ok"`
		Status  string `msgpack:"status"`
		Message string `msgpack:"message"`
	}
	if err := msgpack.Unmarshal(buf, &resp); err != nil {
		return false, "", "", err
	}
	return resp.Ok, resp.Status, resp.Message, nil
}

// Status reports whether the bundled toolchain binary for lang exists under the
// host's runtime dir. It does NOT check the system PATH — that's the cell's
// spawn.process job. path is the absolute path to the bundled binary when present.
func (toolchainAPI) Status(lang string) (present bool, path string, err error) {
	req, err := msgpack.Marshal(struct {
		Lang string `msgpack:"lang"`
	}{Lang: lang})
	if err != nil {
		return false, "", err
	}
	var respPtr, respLen uint32
	code := hostToolchainStatus(
		uint32(uintptr(unsafe.Pointer(&req[0]))), uint32(len(req)),
		uint32(uintptr(unsafe.Pointer(&respPtr))), uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(req)
	if code == 99 {
		return false, "", ErrCapabilityUnavailable
	}
	if code != 0 {
		return false, "", fmt.Errorf("toolchain_status host code %d", code)
	}
	if respLen == 0 {
		return false, "", nil
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	buf := make([]byte, respLen)
	copy(buf, src)
	releaseHostAlloc(respPtr, respLen)
	var resp struct {
		Present bool   `msgpack:"present"`
		Path    string `msgpack:"path"`
	}
	if err := msgpack.Unmarshal(buf, &resp); err != nil {
		return false, "", err
	}
	return resp.Present, resp.Path, nil
}
