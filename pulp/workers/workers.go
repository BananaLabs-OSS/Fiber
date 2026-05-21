// Package workers is the cell-side wrapper for the workers capability
// provided by Pulp-ext-workers. Cell code calls these methods to
// submit background tasks (HTTP requests, etc.) that the host executes
// asynchronously outside the cell's WASM step.
//
//	import "github.com/BananaLabs-OSS/Fiber/pulp/workers"
//
//	id, err := workers.Submit(workers.Task{Type: "http.fetch", Method: "POST", URL: "https://example.com"})
//	// ... later ...
//	res, done, err := workers.Result(id)
//
// The cell's manifest must declare:
//
//	capabilities = ["workers"]
//
// and the host binary must link Pulp-ext-workers via blank import.
package workers

import (
	"errors"
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

//go:wasmimport pulp workers_submit
func hostSubmit(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp workers_submit_fire
func hostSubmitFire(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp workers_result
func hostResult(taskID, resultPtrOut, resultLenOut uint32) uint32

//go:wasmimport pulp workers_cancel
func hostCancel(taskID uint32) uint32

//go:wasmimport pulp workers_pending
func hostPending() uint32

// ErrWorkersQueueFull is returned when the host's inflight queue is full.
// Maps to host code 15.
var ErrWorkersQueueFull = errors.New("pulp/workers: queue full")

// ErrTaskUnknown is returned by Result when the host reports the task
// id is neither in-flight nor in the results cache. Either it never
// existed or its result was already consumed by a previous Result
// call (results are one-shot — terminal statuses are deleted on read).
// Maps to host status code 4.
var ErrTaskUnknown = errors.New("pulp/workers: task unknown")

// firstTaskID mirrors the host-side reservation: task ids start at 100 so
// the small values (1, 2, 3, 15, 99) returned by workers_submit are
// unambiguously error codes.
const firstTaskID = 100

// Host-side status codes returned by workers_result. Must stay in
// lock-step with Pulp-ext-workers/workers.go.
const (
	statusPending  uint32 = 0
	statusComplete uint32 = 1
	statusError    uint32 = 2
	statusPanic    uint32 = 3
	statusUnknown  uint32 = 4
)

// Task describes a unit of work to submit to the host worker pool.
type Task struct {
	Type      string            `msgpack:"type"`
	Method    string            `msgpack:"method,omitempty"`
	URL       string            `msgpack:"url,omitempty"`
	Headers   map[string]string `msgpack:"headers,omitempty"`
	Body      []byte            `msgpack:"body,omitempty"`
	TimeoutMs uint32            `msgpack:"timeout_ms,omitempty"`
}

// TaskResult is the decoded result of a completed task.
type TaskResult struct {
	Status  int               `msgpack:"status"`
	Headers map[string]string `msgpack:"headers,omitempty"`
	Body    []byte            `msgpack:"body,omitempty"`
	Error   string            `msgpack:"error,omitempty"`
}

// FetchRequest is a convenience type for fire-and-forget HTTP requests.
type FetchRequest struct {
	Method  string            `msgpack:"method"`
	URL     string            `msgpack:"url"`
	Headers map[string]string `msgpack:"headers,omitempty"`
	Body    []byte            `msgpack:"body,omitempty"`
}

// Submit submits a task and returns a task ID for polling the result.
// Equivalent to SubmitWithTimeout(req, 0) — relies on the host's default
// per-task timeout.
func Submit(req Task) (uint32, error) {
	return SubmitWithTimeout(req, 0)
}

// SubmitWithTimeout submits a task with an optional per-task timeout
// override. When timeout <= 0 the host's default timeout is used.
func SubmitWithTimeout(req Task, timeout time.Duration) (uint32, error) {
	if timeout > 0 {
		req.TimeoutMs = uint32(timeout / time.Millisecond)
	}
	data, err := msgpack.Marshal(req)
	if err != nil {
		return 0, fmt.Errorf("encode submit: %w", err)
	}
	ret := hostSubmit(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	if ret >= firstTaskID {
		return ret, nil
	}
	return 0, submitCodeToError("workers_submit", ret)
}

// Fire submits a fire-and-forget task. No result tracking.
func Fire(req Task) error {
	data, err := msgpack.Marshal(req)
	if err != nil {
		return fmt.Errorf("encode fire: %w", err)
	}
	code := hostSubmitFire(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	return codeToError("workers_submit_fire", code)
}

// Result polls for a task's result. Returns (result, done, error).
//
// done=false means the task is still in-flight — poll again later.
// done=true means the task has reached a terminal state: either
// success (err=nil, result.Status/Headers/Body populated from the
// HTTP response) or failure (result.Error set).
//
// The host writes its status code into the u32 return value — not a
// distinct error code — so we map it here: statusPending → pending,
// statusComplete → decode body as abi.HTTPResponse, statusError/
// statusPanic → Error field carries the host-reported message,
// statusUnknown → ErrTaskUnknown (task id never existed or result
// was already consumed on a prior Result call).
func Result(taskID uint32) (TaskResult, bool, error) {
	var respPtr, respLen uint32
	status := hostResult(
		taskID,
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	switch status {
	case statusPending:
		return TaskResult{}, false, nil
	case statusUnknown:
		return TaskResult{}, false, ErrTaskUnknown
	case statusComplete:
		if respLen == 0 {
			// Completed with no body — still a terminal state. Return
			// a zero-valued TaskResult with done=true so callers don't
			// retry forever.
			return TaskResult{}, true, nil
		}
		respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
		// The host encodes the completed http.fetch task body as an
		// abi.HTTPResponse — decode into a matching shape. Status,
		// Headers, Body share msgpack keys; Cookies/ID fields are
		// absent from TaskResult by design.
		var decoded struct {
			Status  uint32            `msgpack:"status"`
			Headers map[string]string `msgpack:"headers"`
			Body    []byte            `msgpack:"body"`
		}
		if err := msgpack.Unmarshal(respBytes, &decoded); err != nil {
			return TaskResult{}, true, fmt.Errorf("decode result: %w", err)
		}
		return TaskResult{
			Status:  int(decoded.Status),
			Headers: decoded.Headers,
			Body:    decoded.Body,
		}, true, nil
	case statusError, statusPanic:
		// The host writes the raw error string bytes into the buffer,
		// not a msgpack struct. Surface it via TaskResult.Error so
		// callers can branch on done=true + Error!="" to detect failure.
		var msg string
		if respLen > 0 {
			respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
			msg = string(respBytes)
		}
		if status == statusPanic && msg == "" {
			msg = "task panicked"
		}
		if status == statusError && msg == "" {
			msg = "task failed"
		}
		return TaskResult{Error: msg}, true, nil
	default:
		return TaskResult{}, false, fmt.Errorf("workers_result: unexpected host status %d", status)
	}
}

// Cancel cancels an in-flight task. Returns nil on success, or
// ErrTaskUnknown when the host cannot find the task id in its
// in-flight set (never submitted, already completed, or already
// cancelled — all three collapse to the same "not found" response).
func Cancel(taskID uint32) error {
	code := hostCancel(taskID)
	switch code {
	case 0:
		return nil
	case 1:
		return ErrTaskUnknown
	default:
		return fmt.Errorf("workers_cancel: host code %d", code)
	}
}

// Pending returns the number of in-flight tasks.
func Pending() uint32 {
	return hostPending()
}

// FetchAsync is a convenience wrapper: fire-and-forget HTTP request.
// Equivalent to Fire(Task{Type: "http.fetch", ...}).
func FetchAsync(req FetchRequest) error {
	return Fire(Task{
		Type:    "http.fetch",
		Method:  req.Method,
		URL:     req.URL,
		Headers: req.Headers,
		Body:    req.Body,
	})
}

// submitCodeToError maps workers_submit error returns (< firstTaskID) to errors.
func submitCodeToError(op string, code uint32) error {
	switch code {
	case 0:
		return fmt.Errorf("%s: host rejected task", op)
	case 1:
		return fmt.Errorf("%s: empty request", op)
	case 2:
		return fmt.Errorf("%s: host memory read failed", op)
	case 3:
		return fmt.Errorf("%s: decode error", op)
	case 15:
		return ErrWorkersQueueFull
	case 99:
		return pulp.ErrCapabilityUnavailable
	default:
		return fmt.Errorf("%s: host code %d", op, code)
	}
}

// codeToError maps Pulp-ext-workers host error codes to Go errors.
// 0 = ok, 1 = empty, 2 = mem read, 3 = decode, 15 = queue full, 99 = capability absent.
func codeToError(op string, code uint32) error {
	switch code {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("%s: empty request", op)
	case 2:
		return fmt.Errorf("%s: host memory read failed", op)
	case 3:
		return fmt.Errorf("%s: decode error", op)
	case 15:
		return ErrWorkersQueueFull
	case 99:
		return pulp.ErrCapabilityUnavailable
	default:
		return fmt.Errorf("%s: host code %d", op, code)
	}
}
