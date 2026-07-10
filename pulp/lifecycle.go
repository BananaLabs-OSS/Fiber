package pulp

import (
	"encoding/binary"
	"fmt"
	"sync"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// InitFunc is invoked during pulp_init with the MessagePack-encoded
// [config] table from the manifest. Return a non-zero error to fail
// cell startup.
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
	userTick     TickFunc
	userShutdown ShutdownFunc
)

// OnInit registers the function the cell calls during pulp_init.
// Call this from an init() in your main package. Replacing a previous
// registration is allowed.
func OnInit(fn InitFunc) { userInit = fn }

// OnStep registers the function invoked for each non-tick pulp_step.
func OnStep(fn StepFunc) { userStep = fn }

// TickFunc is invoked on every idle pulp_step (no event), carrying the host
// wall-clock in nanoseconds. This is how a cell runs autonomous, server-side
// periodic work — e.g. firing a timer's deadline with no client connected. It
// runs in the same single-threaded step loop as HTTP/SSE dispatch, so it must
// be fast.
type TickFunc func(wallTimeNanos uint64) error

// OnTick registers the function invoked on each idle (no-event) pulp_step.
func OnTick(fn TickFunc) { userTick = fn }

// OnShutdown registers the function invoked for pulp_shutdown.
func OnShutdown(fn ShutdownFunc) { userShutdown = fn }

// allocTable keeps host-allocated buffers alive until pulp_free is
// called (or the cell exits). Without this, a buffer returned from
// pulpAlloc would be freed by Go's GC the moment pulpAlloc returns —
// the host would then write response bytes into memory the cell
// runtime has already reused for stack/heap, producing garbage on the
// cell side. Observed in practice with Bananagine's FS.List where
// the host encoded a valid 25-byte response but the cell saw 8
// bytes of overwritten pointer data followed by more garbage.
//
// The map is guarded by a mutex because while WASM cells are
// single-threaded, pulpAlloc may be called reentrantly (host → cell
// → host → cell) and the scheduler can interleave goroutines if the
// cell spawns them.
var (
	allocMu    sync.Mutex
	allocTable = map[uint32][]byte{}
)

//go:wasmexport pulp_alloc
func pulpAlloc(size uint32) uint32 {
	if size == 0 {
		return 0
	}
	buf := make([]byte, size)
	ptr := uint32(uintptr(unsafe.Pointer(&buf[0])))
	allocMu.Lock()
	allocTable[ptr] = buf
	allocMu.Unlock()
	return ptr
}

//go:wasmexport pulp_free
func pulpFree(ptr, size uint32) {
	_ = size
	if ptr == 0 {
		return
	}
	allocMu.Lock()
	delete(allocTable, ptr)
	allocMu.Unlock()
}

// Alloc reserves size bytes of cell linear memory and PINS the backing slice
// in the cell's alloc table so Go's GC will not reclaim it until Free (or the
// host's pulp_post_return) releases it. It shares the SAME allocTable as
// pulpAlloc, so the host's pulp_free and the cell's own frees stay consistent.
//
// Alloc is the exported entry the witcell-generated canonical-ABI glue uses to
// build a response pointer tree: lowerResult allocs each record/list/string
// sub-buffer here (pinned), and the generated pulp_post_return tree-frees them
// via Free after the host has lifted the typed value. Returns 0 for size 0.
// Legacy msgpack cells never touch this — the opaque []byte path is unchanged.
func Alloc(size uint32) uint32 { return pulpAlloc(size) }

// Free releases a pointer previously returned by Alloc (or pulpAlloc),
// unpinning it from the alloc table. Used by generated pulp_post_return glue
// to tree-free a lowered canonical-ABI response.
func Free(ptr uint32) { pulpFree(ptr, 0) }

// AllocLive returns the number of buffers currently pinned in the cell's alloc
// table. Diagnostic only: a canonical-ABI cell can export this so a host test
// can assert that a lowered response tree returns to its baseline pin count
// after pulp_post_return (i.e. zero leaked sub-buffers).
func AllocLive() uint32 {
	allocMu.Lock()
	n := uint32(len(allocTable))
	allocMu.Unlock()
	return n
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
func pulpStep(inputPtr, inputLen uint32) (rc int32) {
	// GUARD: recover from any panic in a handler/tick so it fails THIS step only
	// — never traps the whole wasm module. A module trap would kill the cell and
	// 500 every subsequent request (the "cell did not respond" death). With this,
	// a bad request, parse, or missing grammar fails alone; the cell stays alive.
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[pulp] step PANIC recovered — cell survives: %v\n", r)
			rc = 1
		}
	}()
	if inputLen < 20 {
		return 0
	}
	raw := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(inputPtr))), inputLen)

	callNumber := binary.LittleEndian.Uint64(raw[0:8])
	wallTime := binary.LittleEndian.Uint64(raw[8:16])
	payloadLen := binary.LittleEndian.Uint32(raw[16:20])
	if 20+payloadLen > uint32(inputLen) {
		return 1
	}
	if payloadLen == 0 {
		// Idle tick — no event. Drive the cell's autonomous periodic work
		// (server-side timers/deadlines) if a handler is registered.
		if userTick != nil {
			if err := userTick(wallTime); err != nil {
				fmt.Printf("[pulp] tick error: %v\n", err)
				return 1
			}
		}
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
func pulpShutdown() (rc int32) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[pulp] shutdown PANIC recovered: %v\n", r)
			rc = 1
		}
	}()
	if userShutdown == nil {
		return 0
	}
	if err := userShutdown(); err != nil {
		fmt.Printf("[pulp] shutdown error: %v\n", err)
		return 1
	}
	return 0
}
