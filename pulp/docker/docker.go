// Package docker is the cell-side wrapper for the spawn.docker
// capability provided by Pulp-ext-docker. Cell code calls these
// methods to manage Docker containers without touching host imports
// directly.
//
//	import "github.com/BananaLabs-OSS/Fiber/pulp/docker"
//
//	servers, err := docker.List(nil)
//	server, err := docker.Create(docker.CreateRequest{...})
//	err := docker.Destroy("container-id")
//
// The cell's manifest must declare:
//
//	capabilities = ["spawn.docker"]
//
// and the host binary must link Pulp-ext-docker via blank import.
package docker

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

// ---- host imports ----------------------------------------------------------

//go:wasmimport pulp docker_list
func hostList(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_get
func hostGet(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_create
func hostCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_destroy
func hostDestroy(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp docker_restart
func hostRestart(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp docker_exec
func hostExec(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_logs
func hostLogs(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_stats
func hostStats(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_files_read
func hostFilesRead(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_files_write
func hostFilesWrite(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp docker_files_delete
func hostFilesDelete(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp docker_events_poll
func hostEventsPoll(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_stats_all
func hostStatsAll(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_build
func hostBuild(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp docker_build_status
func hostBuildStatus(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

// ---- request / response types ----------------------------------------------

type listRequest struct {
	Filter map[string]string `msgpack:"filter,omitempty"`
}

type PortBinding struct {
	Host      int    `msgpack:"host"`
	Container int    `msgpack:"container"`
	Protocol  string `msgpack:"protocol"`
	Name      string `msgpack:"name,omitempty"`
	Range     string `msgpack:"range,omitempty"`
}

type CreateRequest struct {
	Image          string            `msgpack:"image"`
	Name           string            `msgpack:"name,omitempty"`
	Environment    map[string]string `msgpack:"environment,omitempty"`
	Volumes        map[string]string `msgpack:"volumes,omitempty"`
	Ports          []PortBinding     `msgpack:"ports,omitempty"`
	Network        string            `msgpack:"network,omitempty"`
	IP             string            `msgpack:"ip,omitempty"`
	MemoryLimit    int64             `msgpack:"memory_limit,omitempty"`
	CPULimit       float64           `msgpack:"cpu_limit,omitempty"`
	DiskIOReadBps  int64             `msgpack:"disk_io_read_bps,omitempty"`
	DiskIOWriteBps int64             `msgpack:"disk_io_write_bps,omitempty"`
	DiskSizeLimit  int64             `msgpack:"disk_size_limit,omitempty"`
	PidsLimit      int64             `msgpack:"pids_limit,omitempty"`
	MemorySwap     int64             `msgpack:"memory_swap,omitempty"`
}

type Server struct {
	ID          string         `msgpack:"id" json:"id"`
	Name        string         `msgpack:"name" json:"name"`
	Status      string         `msgpack:"status" json:"status"`
	IP          string         `msgpack:"ip" json:"ip"`
	Ports       map[string]int `msgpack:"ports" json:"ports"`
	CPULimit    float64        `msgpack:"cpu_limit,omitempty" json:"cpu_limit,omitempty"`
	MemoryLimit int64          `msgpack:"memory_limit,omitempty" json:"memory_limit,omitempty"`
}

type ContainerStats struct {
	ContainerID    string  `msgpack:"container_id" json:"container_id"`
	Name           string  `msgpack:"name" json:"name"`
	CPUPercent     float64 `msgpack:"cpu_percent" json:"cpu_percent"`
	MemoryUsed     int64   `msgpack:"memory_used" json:"memory_used"`
	MemoryLimit    int64   `msgpack:"memory_limit" json:"memory_limit"`
	NetRxBytes     int64   `msgpack:"net_rx_bytes" json:"net_rx_bytes"`
	NetTxBytes     int64   `msgpack:"net_tx_bytes" json:"net_tx_bytes"`
	DiskReadBytes  int64   `msgpack:"disk_read_bytes" json:"disk_read_bytes"`
	DiskWriteBytes int64   `msgpack:"disk_write_bytes" json:"disk_write_bytes"`
	Timestamp      int64   `msgpack:"timestamp" json:"timestamp"`
}

type idRequest struct {
	ID string `msgpack:"id"`
}

type nameRequest struct {
	Name string `msgpack:"name"`
}

type filesDeleteRequest struct {
	Container string `msgpack:"container"`
	Path      string `msgpack:"path"`
}

type execRequest struct {
	ID  string   `msgpack:"id"`
	Cmd []string `msgpack:"cmd"`
}

type execResponse struct {
	Output string `msgpack:"output"`
}

type logsRequest struct {
	ID   string `msgpack:"id"`
	Tail int    `msgpack:"tail"`
}

type logsResponse struct {
	Logs string `msgpack:"logs"`
}

type filesReadRequest struct {
	ID   string `msgpack:"id"`
	Path string `msgpack:"path"`
}

type filesWriteRequest struct {
	ID   string `msgpack:"id"`
	Path string `msgpack:"path"`
	Data []byte `msgpack:"data"`
}

type eventsPollRequest struct {
	SinceNanos int64 `msgpack:"since_nanos"`
	Limit      int   `msgpack:"limit"`
}

type Event struct {
	Timestamp   int64  `msgpack:"timestamp"`
	ContainerID string `msgpack:"container_id"`
	Name        string `msgpack:"name"`
	Action      string `msgpack:"action"`
}

type BuildRequest struct {
	BuildArgs map[string]string `msgpack:"build_args,omitempty"`
	ImageTag  string            `msgpack:"image_tag"`
	BuildDir  string            `msgpack:"build_dir"`
}

type BuildStatus struct {
	Building      bool   `msgpack:"building"`
	LastBuildTime int64  `msgpack:"last_build_time"`
	LastError     string `msgpack:"last_error,omitempty"`
}

// ---- public API ------------------------------------------------------------

func List(filter map[string]string) ([]Server, error) {
	data, err := msgpack.Marshal(listRequest{Filter: filter})
	if err != nil {
		return nil, fmt.Errorf("encode list: %w", err)
	}
	var respPtr, respLen uint32
	code := hostList(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("docker_list", code); err != nil {
		return nil, err
	}
	if respLen == 0 {
		return nil, nil
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	out := make([]byte, len(respBytes))
	copy(out, respBytes)
	pulp.ReleaseHostAlloc(respPtr, respLen)
	var servers []Server
	if err := msgpack.Unmarshal(out, &servers); err != nil {
		return nil, fmt.Errorf("decode list: %w", err)
	}
	return servers, nil
}

func Get(name string) (*Server, error) {
	data, err := msgpack.Marshal(nameRequest{Name: name})
	if err != nil {
		return nil, fmt.Errorf("encode get: %w", err)
	}
	var respPtr, respLen uint32
	code := hostGet(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("docker_get", code); err != nil {
		return nil, err
	}
	if respLen == 0 {
		return nil, pulp.ErrNotFound
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	buf := make([]byte, respLen)
	copy(buf, respBytes)
	pulp.ReleaseHostAlloc(respPtr, respLen)
	var server Server
	if err := msgpack.Unmarshal(buf, &server); err != nil {
		return nil, fmt.Errorf("decode get: %w", err)
	}
	return &server, nil
}

func Create(req CreateRequest) (*Server, error) {
	data, err := msgpack.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode create: %w", err)
	}
	var respPtr, respLen uint32
	code := hostCreate(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("docker_create", code); err != nil {
		return nil, err
	}
	if respLen == 0 {
		return nil, fmt.Errorf("docker_create: empty response")
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	buf := make([]byte, respLen)
	copy(buf, respBytes)
	pulp.ReleaseHostAlloc(respPtr, respLen)
	var server Server
	if err := msgpack.Unmarshal(buf, &server); err != nil {
		return nil, fmt.Errorf("decode create: %w", err)
	}
	return &server, nil
}

func Destroy(id string) error {
	data, err := msgpack.Marshal(idRequest{ID: id})
	if err != nil {
		return fmt.Errorf("encode destroy: %w", err)
	}
	code := hostDestroy(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	return codeToError("docker_destroy", code)
}

func Restart(id string) error {
	data, err := msgpack.Marshal(idRequest{ID: id})
	if err != nil {
		return fmt.Errorf("encode restart: %w", err)
	}
	code := hostRestart(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	return codeToError("docker_restart", code)
}

func Exec(id string, cmd []string) (string, error) {
	data, err := msgpack.Marshal(execRequest{ID: id, Cmd: cmd})
	if err != nil {
		return "", fmt.Errorf("encode exec: %w", err)
	}
	var respPtr, respLen uint32
	code := hostExec(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("docker_exec", code); err != nil {
		return "", err
	}
	if respLen == 0 {
		return "", nil
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	buf := make([]byte, respLen)
	copy(buf, respBytes)
	pulp.ReleaseHostAlloc(respPtr, respLen)
	var resp execResponse
	if err := msgpack.Unmarshal(buf, &resp); err != nil {
		return "", fmt.Errorf("decode exec: %w", err)
	}
	return resp.Output, nil
}

func Logs(id string, tail int) (string, error) {
	data, err := msgpack.Marshal(logsRequest{ID: id, Tail: tail})
	if err != nil {
		return "", fmt.Errorf("encode logs: %w", err)
	}
	var respPtr, respLen uint32
	code := hostLogs(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("docker_logs", code); err != nil {
		return "", err
	}
	if respLen == 0 {
		return "", nil
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	buf := make([]byte, respLen)
	copy(buf, respBytes)
	pulp.ReleaseHostAlloc(respPtr, respLen)
	var resp logsResponse
	if err := msgpack.Unmarshal(buf, &resp); err != nil {
		return "", fmt.Errorf("decode logs: %w", err)
	}
	return resp.Logs, nil
}

func Stats(id string) (*ContainerStats, error) {
	data, err := msgpack.Marshal(idRequest{ID: id})
	if err != nil {
		return nil, fmt.Errorf("encode stats: %w", err)
	}
	var respPtr, respLen uint32
	code := hostStats(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("docker_stats", code); err != nil {
		return nil, err
	}
	if respLen == 0 {
		return nil, nil
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	buf := make([]byte, respLen)
	copy(buf, respBytes)
	pulp.ReleaseHostAlloc(respPtr, respLen)
	var stats ContainerStats
	if err := msgpack.Unmarshal(buf, &stats); err != nil {
		return nil, fmt.Errorf("decode stats: %w", err)
	}
	return &stats, nil
}

func FilesRead(id, path string) ([]byte, error) {
	data, err := msgpack.Marshal(filesReadRequest{ID: id, Path: path})
	if err != nil {
		return nil, fmt.Errorf("encode files_read: %w", err)
	}
	var respPtr, respLen uint32
	code := hostFilesRead(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("docker_files_read", code); err != nil {
		return nil, err
	}
	if respLen == 0 {
		return nil, nil
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	out := make([]byte, len(respBytes))
	copy(out, respBytes)
	pulp.ReleaseHostAlloc(respPtr, respLen)
	return out, nil
}

func FilesWrite(id, path string, data []byte) error {
	reqData, err := msgpack.Marshal(filesWriteRequest{ID: id, Path: path, Data: data})
	if err != nil {
		return fmt.Errorf("encode files_write: %w", err)
	}
	code := hostFilesWrite(uint32(uintptr(unsafe.Pointer(&reqData[0]))), uint32(len(reqData)))
	runtime.KeepAlive(reqData)
	return codeToError("docker_files_write", code)
}

func FilesDelete(container, path string) error {
	reqData, err := msgpack.Marshal(filesDeleteRequest{Container: container, Path: path})
	if err != nil {
		return fmt.Errorf("encode files_delete: %w", err)
	}
	code := hostFilesDelete(uint32(uintptr(unsafe.Pointer(&reqData[0]))), uint32(len(reqData)))
	runtime.KeepAlive(reqData)
	return codeToError("docker_files_delete", code)
}

func EventsPoll(sinceNanos int64, limit int) ([]Event, error) {
	data, err := msgpack.Marshal(eventsPollRequest{SinceNanos: sinceNanos, Limit: limit})
	if err != nil {
		return nil, fmt.Errorf("encode events_poll: %w", err)
	}
	var respPtr, respLen uint32
	code := hostEventsPoll(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("docker_events_poll", code); err != nil {
		return nil, err
	}
	if respLen == 0 {
		return nil, nil
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	buf := make([]byte, respLen)
	copy(buf, respBytes)
	pulp.ReleaseHostAlloc(respPtr, respLen)
	var events []Event
	if err := msgpack.Unmarshal(buf, &events); err != nil {
		return nil, fmt.Errorf("decode events_poll: %w", err)
	}
	return events, nil
}

func StatsAll() ([]ContainerStats, error) {
	// No request data needed — pass empty
	data := []byte{0x80} // msgpack empty map
	var respPtr, respLen uint32
	code := hostStatsAll(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("docker_stats_all", code); err != nil {
		return nil, err
	}
	if respLen == 0 {
		return nil, nil
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	buf := make([]byte, respLen)
	copy(buf, respBytes)
	pulp.ReleaseHostAlloc(respPtr, respLen)
	var stats []ContainerStats
	if err := msgpack.Unmarshal(buf, &stats); err != nil {
		return nil, fmt.Errorf("decode stats_all: %w", err)
	}
	return stats, nil
}

func Build(req BuildRequest) error {
	data, err := msgpack.Marshal(req)
	if err != nil {
		return fmt.Errorf("encode build: %w", err)
	}
	code := hostBuild(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	return codeToError("docker_build", code)
}

func GetBuildStatus() (*BuildStatus, error) {
	data := []byte{0x80}
	var respPtr, respLen uint32
	code := hostBuildStatus(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("docker_build_status", code); err != nil {
		return nil, err
	}
	if respLen == 0 {
		return &BuildStatus{}, nil
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	buf := make([]byte, respLen)
	copy(buf, respBytes)
	pulp.ReleaseHostAlloc(respPtr, respLen)
	var status BuildStatus
	if err := msgpack.Unmarshal(buf, &status); err != nil {
		return nil, fmt.Errorf("decode build_status: %w", err)
	}
	return &status, nil
}

// ---- error mapping ---------------------------------------------------------

// ErrBuildInProgress is returned by Build when another build is
// already running. Callers can branch on it with errors.Is to back off
// and retry.
var ErrBuildInProgress = errors.New("pulp/docker: build already in progress")

// ErrInvalidRequest is returned when a host call was made with a
// missing or empty required field (e.g. empty container ID or path).
// Usually indicates a bug in the cell, not a transient failure.
var ErrInvalidRequest = errors.New("pulp/docker: invalid request")

func codeToError(op string, code uint32) error {
	switch code {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("%s: %w", op, ErrInvalidRequest)
	case 6:
		return pulp.ErrNotFound
	case 10:
		return fmt.Errorf("%s: docker provider not available", op)
	case 11:
		return fmt.Errorf("%s: %w", op, ErrBuildInProgress)
	case 4:
		return fmt.Errorf("%s: docker api error", op)
	case 99:
		return fmt.Errorf("%s: %w", op, pulp.ErrCapabilityUnavailable)
	default:
		return fmt.Errorf("%s host code %d", op, code)
	}
}
