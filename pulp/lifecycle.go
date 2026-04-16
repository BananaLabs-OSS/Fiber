package pulp

import (
	"encoding/binary"
	"fmt"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// InitFunc is invoked during pulp_init with the MessagePack-encoded
// [config] table from the manifest. Return a non-zero error to fail
// plugin startup.
type InitFunc func(config []byte) error

// StepFunc is invoked on every pulp_step that carries an event. Idle
// ticks (no payload) do not invoke it. Implementations must be fast —
// HTTP requests expect a response via HTTP.Respond before StepFunc
// returns, or the host will time out the caller.
type StepFunc func(event StepEvent) error

// ShutdownFunc is invoked when the host calls pulp_shutdown (typically
// on SIGINT / SIGTERM). Intended for flushing state, not for blocking
// teardown — the host caps shutdown at 5 seconds.
type ShutdownFunc func() error

var (
	userInit     InitFunc
	userStep     StepFunc
	userShutdown ShutdownFunc
)

// OnInit registers the function the plugin calls during pulp_init.
// Call this from an init() in your main package. Replacing a previous
// registration is allowed.
func OnInit(fn InitFunc) { userInit = fn }

// OnStep registers the function invoked for each non-tick pulp_step.
func OnStep(fn StepFunc) { userStep = fn }

// OnShutdown registers the function invoked for pulp_shutdown.
func OnShutdown(fn ShutdownFunc) { userShutdown = fn }

//go:wasmexport pulp_alloc
func pulpAlloc(size uint32) uint32 {
	if size == 0 {
		return 0
	}
	buf := make([]byte, size)
	return uint32(uintptr(unsafe.Pointer(&buf[0])))
}

//go:wasmexport pulp_free
func pulpFree(ptr, size uint32) {
	_ = ptr
	_ = size
}

//go:wasmexport pulp_init
func pulpInit(cfgPtr, cfgLen uint32) int32 {
	var config []byte
	if cfgLen > 0 {
		config = unsafe.Slice((*byte)(unsafe.Pointer(uintptr(cfgPtr))), cfgLen)
	}
	if userInit == nil {
		return 0
	}
	if err := userInit(config); err != nil {
		fmt.Printf("[pulp] init error: %v\n", err)
		return 1
	}
	return 0
}

//go:wasmexport pulp_step
func pulpStep(inputPtr, inputLen uint32) int32 {
	if inputLen < 20 {
		return 0
	}
	raw := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(inputPtr))), inputLen)

	callNumber := binary.LittleEndian.Uint64(raw[0:8])
	wallTime := binary.LittleEndian.Uint64(raw[8:16])
	payloadLen := binary.LittleEndian.Uint32(raw[16:20])
	if payloadLen == 0 {
		// Tick — no event. Stay silent, give idle callbacks in a
		// future release if anyone needs them.
		return 0
	}

	payload := raw[20 : 20+payloadLen]
	var ev StepEvent
	if err := msgpack.Unmarshal(payload, &ev); err != nil {
		fmt.Printf("[pulp] step event decode error: %v\n", err)
		return 0
	}
	ev.WallTime = wallTime
	ev.CallNumber = callNumber

	if userStep == nil {
		return 0
	}
	if err := userStep(ev); err != nil {
		fmt.Printf("[pulp] step error: %v\n", err)
		return 1
	}
	return 0
}

//go:wasmexport pulp_shutdown
func pulpShutdown() int32 {
	if userShutdown == nil {
		return 0
	}
	if err := userShutdown(); err != nil {
		fmt.Printf("[pulp] shutdown error: %v\n", err)
		return 1
	}
	return 0
}
