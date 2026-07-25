package pulp

// Cell-side client for the spawn.process capability (Pulp-ext-process). Lets a
// cell run a host command — `go build`, `git`, `claude`, or `<host-exe> ctl
// reload <self>` — as an ordinary OS process. The cell must declare
// "spawn.process" in its manifest; the host enforces a no-shell argv exec plus
// binary + working-dir allowlists.
//
// The host model is async (submit → poll → cancel). Run() wraps that into a
// single blocking call for the common case; it polls inside the current step,
// so size the cell's call_timeout_ms to the longest command (a cold build).

import (
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// Process groups the host-import wrappers for the spawn.process capability.
var Process = processAPI{}

type processAPI struct{}

// ProcessResult is the outcome of a finished command. A non-zero ExitCode with
// an empty Error means the command ran and exited non-zero (normal); a
// non-empty Error means it failed to start, timed out, or was killed.
type ProcessResult struct {
	ExitCode int    `msgpack:"exit_code"`
	Stdout   []byte `msgpack:"stdout"`
	Stderr   []byte `msgpack:"stderr"`
	Error    string `msgpack:"error,omitempty"`
}

// RunRequest is the command to run. Argv is exec'd directly (no shell). Dir, if
// set, must resolve under a host-configured run root. Env overlays the host env.
type RunRequest struct {
	Argv      []string          `msgpack:"argv"`
	Dir       string            `msgpack:"dir,omitempty"`
	Env       map[string]string `msgpack:"env,omitempty"`
	TimeoutMs uint32            `msgpack:"timeout_ms,omitempty"`
}

// NoTimeout marks a LONG-LIVED process (a screen-stream helper, a port-forward
// listener) that must NOT be killed by the default 5-minute process timeout. It
// still terminates on Cancel or when the host shuts down. Set RunRequest.TimeoutMs
// to this for such spawns.
const NoTimeout uint32 = 0xFFFFFFFF

// Host status codes returned by process_result.
const (
	procStatusPending  = 0
	procStatusComplete = 1
	procStatusError    = 2
	procStatusUnknown  = 4
)

// firstTaskID mirrors the host: an id is >= 100, so a smaller return value from
// process_run is an error code, not a task id.
const firstTaskID = 100

// Submit queues a command and returns its task id, or an error if the host
// rejected it (capability absent, guard denial, queue/cell full).
func (processAPI) Submit(req RunRequest) (uint32, error) {
	reqBytes, err := msgpack.Marshal(req)
	if err != nil {
		return 0, fmt.Errorf("encode process_run req: %w", err)
	}
	code := hostProcessRun(
		uint32(uintptr(unsafe.Pointer(&reqBytes[0]))),
		uint32(len(reqBytes)),
	)
	runtime.KeepAlive(reqBytes)
	if code >= firstTaskID {
		return code, nil
	}
	switch code {
	case 5:
		return 0, fmt.Errorf("process: binary not in allowed set")
	case 6:
		return 0, fmt.Errorf("process: working dir not under an allowed root")
	case 99:
		return 0, ErrCapabilityUnavailable
	default:
		return 0, fmt.Errorf("process_run host code %d", code)
	}
}

// Poll checks a task. ok=false means still pending. On completion it returns
// the decoded result (which itself carries ExitCode/Error).
func (processAPI) Poll(taskID uint32) (res ProcessResult, ok bool, err error) {
	var outPtr, outLen uint32
	status := hostProcessResult(
		taskID,
		uint32(uintptr(unsafe.Pointer(&outPtr))),
		uint32(uintptr(unsafe.Pointer(&outLen))),
	)
	runtime.KeepAlive(&outPtr)
	runtime.KeepAlive(&outLen)
	switch status {
	case procStatusPending:
		return ProcessResult{}, false, nil
	case procStatusUnknown:
		return ProcessResult{}, false, fmt.Errorf("process: unknown task %d", taskID)
	}
	// complete or error: a payload was written (msgpack ProcessResult, or a raw
	// error string on an internal failure).
	if outLen == 0 {
		return ProcessResult{}, true, nil
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(outPtr))), outLen)
	buf := make([]byte, outLen)
	copy(buf, src)
	releaseHostAlloc(outPtr, outLen)
	if status == procStatusError {
		// Internal error path: payload may be a raw string rather than a result.
		if e := msgpack.Unmarshal(buf, &res); e == nil {
			return res, true, nil
		}
		return ProcessResult{}, true, fmt.Errorf("process: %s", string(buf))
	}
	if e := msgpack.Unmarshal(buf, &res); e != nil {
		return ProcessResult{}, true, fmt.Errorf("decode process result: %w", e)
	}
	return res, true, nil
}

// Cancel signals a running task.
func (processAPI) Cancel(taskID uint32) error {
	if code := hostProcessCancel(taskID); code != 0 {
		return fmt.Errorf("process_cancel host code %d", code)
	}
	return nil
}

// Pending returns the number of in-flight tasks across this cell.
func (processAPI) Pending() uint32 { return hostProcessPending() }

// Run submits a command and blocks (polling) until it finishes, returning the
// result. It polls inside the calling step, so the cell's call_timeout_ms must
// cover the command's worst-case duration. pollEvery defaults to 25ms.
func (processAPI) Run(req RunRequest) (ProcessResult, error) {
	id, err := Process.Submit(req)
	if err != nil {
		return ProcessResult{}, err
	}
	for {
		res, ok, err := Process.Poll(id)
		if err != nil {
			return ProcessResult{}, err
		}
		if ok {
			return res, nil
		}
		time.Sleep(25 * time.Millisecond)
	}
}
