//go:build wasip1

package pulp

// Cell-side client for the spawn.confine capability (Pulp-ext-confine). Lets a
// cell launch a host command with OS-level filesystem confinement applied
// (Landlock on Linux, AppContainer on Windows). The cell must declare
// "spawn.confine" in its manifest.
//
// The host model is async (submit → poll → cancel), matching spawn.process.
// Run() wraps that into a blocking call for the common case.

import (
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// Confine groups the host-import wrappers for the spawn.confine capability.
var Confine = confineAPI{}

type confineAPI struct{}

// ErrConfineUnavailable is returned when the host stubbed the capability —
// the cell did not declare "spawn.confine" in its manifest capabilities list.
var ErrConfineUnavailable = fmt.Errorf("pulp: spawn.confine unavailable (declare it in cell manifest)")

// ConfineRequest describes a confined process launch. Argv is exec'd directly
// (no shell). Root is the allowed read-write root. ReadOnly / ReadWrite refine
// the policy. NetAllow is reserved for future network filtering. Env overlays
// the host env; Dir sets the working directory.
type ConfineRequest struct {
	Argv      []string          `msgpack:"argv"`
	Root      string            `msgpack:"root"`
	ReadOnly  []string          `msgpack:"read_only,omitempty"`
	ReadWrite []string          `msgpack:"read_write,omitempty"`
	NetAllow  []string          `msgpack:"net_allow,omitempty"`
	Env       map[string]string `msgpack:"env,omitempty"`
	Dir       string            `msgpack:"dir,omitempty"`
}

// ConfineResult is the outcome of a finished confined process.
type ConfineResult struct {
	ExitCode int    `msgpack:"exit_code"`
	Error    string `msgpack:"error,omitempty"`
}

// CagedPolicy describes a composed "caged launch": jail PATH + secret env
// injection + network egress confinement + filesystem confinement applied to a
// single native child launch, in ONE capability call. It mirrors
// Pulp-ext-confine/caged.CagedPolicy on the wire.
//
// Secrets maps codename → secret VALUE; values are injected into the child
// process environment only and are NEVER returned in CagedResult.
type CagedPolicy struct {
	Argv      []string          `msgpack:"argv"`
	Root      string            `msgpack:"root"`
	ReadOnly  []string          `msgpack:"read_only,omitempty"`
	ReadWrite []string          `msgpack:"read_write,omitempty"`
	NetAllow  []string          `msgpack:"net_allow,omitempty"`
	JailBins  []string          `msgpack:"jail_bins,omitempty"`
	Secrets   map[string]string `msgpack:"secrets,omitempty"`
	Env       map[string]string `msgpack:"env,omitempty"`
	Dir       string            `msgpack:"dir,omitempty"`
}

// CagedAuditEvent is one structured record of a confinement decision made while
// composing the caged launch (fs / net / jail / secret / launch). It never
// contains secret values.
type CagedAuditEvent struct {
	Kind   string `msgpack:"kind"`
	Detail string `msgpack:"detail"`
	Denied bool   `msgpack:"denied,omitempty"`
}

// CagedResult is the outcome of a composed caged launch.
type CagedResult struct {
	ExitCode    int               `msgpack:"exit_code"`
	Error       string            `msgpack:"error,omitempty"`
	AuditEvents []CagedAuditEvent `msgpack:"audit_events,omitempty"`
}

const (
	confineStatusPending  = uint32(0)
	confineStatusComplete = uint32(1)
	confineStatusError    = uint32(2)
	confineStatusUnknown  = uint32(4)

	confineFirstTaskID = uint32(100)
)

//go:wasmimport pulp confine_spawn
func hostConfineSpawn(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp confine_result
func hostConfineResult(taskID, outPtrOut, outLenOut uint32) uint32

//go:wasmimport pulp confine_cancel
func hostConfineCancel(taskID uint32) uint32

//go:wasmimport pulp caged_run
func hostCagedRun(reqPtr, reqLen uint32) uint32

// Submit queues a confined launch and returns its task id, or an error if the
// host rejected it (capability absent, queue/cell full, invalid request).
func (confineAPI) Submit(req ConfineRequest) (uint32, error) {
	reqBytes, err := msgpack.Marshal(req)
	if err != nil {
		return 0, fmt.Errorf("encode confine_spawn req: %w", err)
	}
	code := hostConfineSpawn(
		uint32(uintptr(unsafe.Pointer(&reqBytes[0]))),
		uint32(len(reqBytes)),
	)
	runtime.KeepAlive(reqBytes)
	if code >= confineFirstTaskID {
		return code, nil
	}
	if code == 99 {
		return 0, ErrConfineUnavailable
	}
	return 0, fmt.Errorf("confine_spawn host code %d", code)
}

// Poll checks a task. ok=false means still pending. On completion it returns
// the decoded result (ExitCode / Error).
func (confineAPI) Poll(taskID uint32) (res ConfineResult, ok bool, err error) {
	var outPtr, outLen uint32
	status := hostConfineResult(
		taskID,
		uint32(uintptr(unsafe.Pointer(&outPtr))),
		uint32(uintptr(unsafe.Pointer(&outLen))),
	)
	runtime.KeepAlive(&outPtr)
	runtime.KeepAlive(&outLen)
	switch status {
	case confineStatusPending:
		return ConfineResult{}, false, nil
	case confineStatusUnknown:
		return ConfineResult{}, false, fmt.Errorf("confine: unknown task %d", taskID)
	}
	if outLen == 0 {
		return ConfineResult{}, true, nil
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(outPtr))), outLen)
	buf := make([]byte, outLen)
	copy(buf, src)
	releaseHostAlloc(outPtr, outLen)
	if status == confineStatusError {
		if e := msgpack.Unmarshal(buf, &res); e == nil {
			return res, true, nil
		}
		return ConfineResult{}, true, fmt.Errorf("confine: %s", string(buf))
	}
	if e := msgpack.Unmarshal(buf, &res); e != nil {
		return ConfineResult{}, true, fmt.Errorf("decode confine result: %w", e)
	}
	return res, true, nil
}

// Cancel signals a running task.
func (confineAPI) Cancel(taskID uint32) error {
	if code := hostConfineCancel(taskID); code != 0 {
		return fmt.Errorf("confine_cancel host code %d", code)
	}
	return nil
}

// Run submits a confined launch and blocks (polling every 25ms) until it
// finishes. The cell's call_timeout_ms must cover the child's worst-case
// duration.
func (confineAPI) Run(req ConfineRequest) (ConfineResult, error) {
	id, err := Confine.Submit(req)
	if err != nil {
		return ConfineResult{}, err
	}
	for {
		res, ok, err := Confine.Poll(id)
		if err != nil {
			return ConfineResult{}, err
		}
		if ok {
			return res, nil
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// SubmitCaged queues a composed caged launch (jail + secrets + egress + FS
// confinement) and returns its task id, or an error if the host rejected it.
// The returned id is polled with the same mechanism as Confine.Poll (task ids
// are shared across confine_spawn and caged_run on the host).
func (confineAPI) SubmitCaged(policy CagedPolicy) (uint32, error) {
	reqBytes, err := msgpack.Marshal(policy)
	if err != nil {
		return 0, fmt.Errorf("encode caged_run req: %w", err)
	}
	code := hostCagedRun(
		uint32(uintptr(unsafe.Pointer(&reqBytes[0]))),
		uint32(len(reqBytes)),
	)
	runtime.KeepAlive(reqBytes)
	if code >= confineFirstTaskID {
		return code, nil
	}
	if code == 99 {
		return 0, ErrConfineUnavailable
	}
	return 0, fmt.Errorf("caged_run host code %d", code)
}

// PollCaged checks a caged task. ok=false means still pending. On completion it
// returns the decoded CagedResult (ExitCode / Error / AuditEvents). Secret
// values are never present in the result.
func (confineAPI) PollCaged(taskID uint32) (res CagedResult, ok bool, err error) {
	var outPtr, outLen uint32
	status := hostConfineResult(
		taskID,
		uint32(uintptr(unsafe.Pointer(&outPtr))),
		uint32(uintptr(unsafe.Pointer(&outLen))),
	)
	runtime.KeepAlive(&outPtr)
	runtime.KeepAlive(&outLen)
	switch status {
	case confineStatusPending:
		return CagedResult{}, false, nil
	case confineStatusUnknown:
		return CagedResult{}, false, fmt.Errorf("caged: unknown task %d", taskID)
	}
	if outLen == 0 {
		return CagedResult{}, true, nil
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(outPtr))), outLen)
	buf := make([]byte, outLen)
	copy(buf, src)
	releaseHostAlloc(outPtr, outLen)
	if status == confineStatusError {
		if e := msgpack.Unmarshal(buf, &res); e == nil {
			return res, true, nil
		}
		return CagedResult{}, true, fmt.Errorf("caged: %s", string(buf))
	}
	if e := msgpack.Unmarshal(buf, &res); e != nil {
		return CagedResult{}, true, fmt.Errorf("decode caged result: %w", e)
	}
	return res, true, nil
}

// RunCaged submits a composed caged launch and blocks (polling every 25ms)
// until the child exits. The cell's call_timeout_ms must cover the child's
// worst-case duration. The returned CagedResult never contains secret values.
func (confineAPI) RunCaged(policy CagedPolicy) (CagedResult, error) {
	id, err := Confine.SubmitCaged(policy)
	if err != nil {
		return CagedResult{}, err
	}
	for {
		res, ok, err := Confine.PollCaged(id)
		if err != nil {
			return CagedResult{}, err
		}
		if ok {
			return res, nil
		}
		time.Sleep(25 * time.Millisecond)
	}
}
