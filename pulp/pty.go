package pulp

// Cell-side client for the spawn.pty capability (Pulp-ext-pty). The cell opens
// a host PTY for a chosen shell, forwards keystrokes + resizes, and receives the
// shell's output as "pty.output" step events (decode with DecodePTYOutput). The
// cell must declare "spawn.pty" in its manifest.

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// PTY groups the host-import wrappers for the spawn.pty capability.
var PTY = ptyAPI{}

type ptyAPI struct{}

// PTYOutput is the payload of a "pty.output" step event: a chunk of a PTY's
// output stream, tagged with the PTY id it came from.
type PTYOutput struct {
	PtyID uint32 `msgpack:"pty_id"`
	Data  []byte `msgpack:"data"`
}

//go:wasmimport pulp pty_open
func hostPTYOpen(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp pty_write
func hostPTYWrite(id, dataPtr, dataLen uint32) uint32

//go:wasmimport pulp pty_resize
func hostPTYResize(id, cols, rows uint32) uint32

//go:wasmimport pulp pty_close
func hostPTYClose(id uint32) uint32

//go:wasmimport pulp pty_alive
func hostPTYAlive(id uint32) uint32

// PTYOpenRequest selects what a host PTY runs. Shell picks a known program
// ("cmd"/"powershell"/"pwsh"/bash/sh, "" = OS default, or "agent" = the Claude
// Code CLI). Args are appended to the resolved command line (e.g. the agent's
// --settings). Dir is the working directory the program starts in.
type PTYOpenRequest struct {
	Shell   string   `msgpack:"shell"`
	Args    []string `msgpack:"args,omitempty"`
	Dir     string   `msgpack:"dir,omitempty"`
	Persist bool     `msgpack:"persist,omitempty"` // keep the process alive across a cell ↻ reload (agent/named panes)
}

// Open starts a host PTY running the given shell ("cmd"/"powershell"/"pwsh" on
// Windows, "bash"/"sh" on unix; "" = OS default) and returns its id.
func (a ptyAPI) Open(shell string) (uint32, error) {
	return a.OpenOpts(PTYOpenRequest{Shell: shell})
}

// OpenOpts starts a host PTY from a full request (shell + args + working dir).
func (ptyAPI) OpenOpts(reqv PTYOpenRequest) (uint32, error) {
	req, err := msgpack.Marshal(reqv)
	if err != nil {
		return 0, err
	}
	var respPtr, respLen uint32
	code := hostPTYOpen(
		uint32(uintptr(unsafe.Pointer(&req[0]))),
		uint32(len(req)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(req)
	if code == 99 {
		return 0, ErrCapabilityUnavailable
	}
	if code != 0 {
		return 0, fmt.Errorf("pty_open host code %d", code)
	}
	if respLen == 0 {
		return 0, fmt.Errorf("pty_open: empty response")
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	buf := make([]byte, respLen)
	copy(buf, src)
	releaseHostAlloc(respPtr, respLen)
	var resp struct {
		PtyID uint32 `msgpack:"pty_id"`
	}
	if err := msgpack.Unmarshal(buf, &resp); err != nil {
		return 0, err
	}
	return resp.PtyID, nil
}

// Write sends input bytes (keystrokes) to a PTY.
func (ptyAPI) Write(id uint32, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	code := hostPTYWrite(id, uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	if code != 0 {
		return fmt.Errorf("pty_write host code %d", code)
	}
	return nil
}

// Resize sets a PTY's window size.
func (ptyAPI) Resize(id uint32, cols, rows int) error {
	if code := hostPTYResize(id, uint32(cols), uint32(rows)); code != 0 {
		return fmt.Errorf("pty_resize host code %d", code)
	}
	return nil
}

// Close terminates a PTY.
func (ptyAPI) Close(id uint32) error {
	if code := hostPTYClose(id); code != 0 {
		return fmt.Errorf("pty_close host code %d", code)
	}
	return nil
}

// Alive reports whether a PTY id still has a running process. False after the
// shell/agent exited, after pty_close, or after a host restart wiped all sessions.
// Lets the cell tell a reattach (process survived a cell reload) from a respawn.
func (ptyAPI) Alive(id uint32) bool {
	return hostPTYAlive(id) == 1
}

// DecodePTYOutput decodes a "pty.output" step event payload.
func DecodePTYOutput(payload []byte) (PTYOutput, error) {
	var o PTYOutput
	err := msgpack.Unmarshal(payload, &o)
	return o, err
}
