package pulp

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"unsafe"
)

// Sibling-call API — in-process cell-to-cell calls through the
// Pulp host. Cell A registers providers via Provide(); cell B
// invokes them via Call(). No HTTP hop; msgpack in, msgpack out.
//
//	// Cell A (provides = ["auth.verify"]):
//	func main() {
//	    pulp.Provide("auth.verify", func(args []byte) ([]byte, error) {
//	        // decode args, do work, return encoded response
//	        return []byte("ok"), nil
//	    })
//	    pulp.OnStep(...)
//	}
//
//	// Cell B (consumes = ["auth.verify"] or depends_on = ["A"]):
//	resp, err := pulp.Call("A", "auth.verify", myArgs)
//
// The cell's manifest must declare `consumes = [...]` naming either
// the target cell or one of its `provides` entries. The host rejects
// unauthorized calls with error code 11.

// Call invokes funcName on target cell with args, returning the
// cell's response bytes. The host enforces that this cell's
// manifest declares `target` under consumes or depends_on.
func Call(target, funcName string, args []byte) ([]byte, error) {
	if target == "" || funcName == "" {
		return nil, fmt.Errorf("pulp.Call: target and funcName required")
	}
	targetBytes := []byte(target)
	nameBytes := []byte(funcName)

	var argsPtr, argsLen uint32
	if len(args) > 0 {
		argsPtr = uint32(uintptr(unsafe.Pointer(&args[0])))
		argsLen = uint32(len(args))
	}

	var respPtr, respLen uint32
	code := hostPulpCall(
		uint32(uintptr(unsafe.Pointer(&targetBytes[0]))), uint32(len(targetBytes)),
		uint32(uintptr(unsafe.Pointer(&nameBytes[0]))), uint32(len(nameBytes)),
		argsPtr, argsLen,
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(targetBytes)
	runtime.KeepAlive(nameBytes)
	runtime.KeepAlive(args)
	if code != 0 {
		return nil, siblingCodeError(code)
	}

	if respLen == 0 {
		return nil, nil
	}
	resp := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	out := make([]byte, len(resp))
	copy(out, resp)
	// Release the target cell's buffer — we've copied into our own
	// GC-owned slice. Without this, every sibling call leaks whatever
	// buffer the provider registered in its alloc table.
	pulpFree(respPtr, respLen)
	return out, nil
}

// Provider is the function signature a cell registers with Provide.
// It receives the raw msgpack-encoded args from the caller and returns
// the raw msgpack-encoded response (or an error — the host returns a
// nonzero code to the caller in that case).
type Provider func(args []byte) ([]byte, error)

var (
	providersMu sync.RWMutex
	providers   = map[string]Provider{}
)

// Provide registers a handler for a sibling-call function name. Must be
// called before the cell begins serving step events (typically in
// OnInit). Overwrites any prior registration with the same name.
func Provide(name string, handler Provider) {
	providersMu.Lock()
	providers[name] = handler
	providersMu.Unlock()
}

// RawProvider is the TYPED, canonical-ABI alternative to Provider. Where a
// Provider takes/returns msgpack []byte, a RawProvider works directly on the
// cell's linear memory: it LIFTS the request straight out of argsPtr (via the
// witcell-generated liftRequest), runs the typed domain handler, LOWERS the
// result as a pointer tree pinned in the cell's alloc table (generated
// lowerResult calling Alloc), and writes the (respPtr, respLen) out-params.
//
// A cell that registers a RawProvider must ALSO export pulp_post_return (the
// generated cabi_post) so the host can tree-free the response after it has
// lifted the typed value while the tree is still pinned. Returns 0 on success,
// nonzero to signal an error to the caller (same code space as dispatchOnCall).
//
// This is emitted by the witcell generator; hand-written cells keep using
// Provider. The two paths coexist — see ProvideRaw / dispatchOnCall.
type RawProvider func(argsPtr, argsLen, respPtrOut, respLenOut uint32) uint32

var (
	rawProvidersMu sync.RWMutex
	rawProviders   = map[string]RawProvider{}
)

// ProvideRaw registers a canonical-ABI (witcell) handler for a sibling-call
// function name. It sits ALONGSIDE Provide, additively: dispatchOnCall checks
// raw providers first and falls back to the msgpack providers, so registering
// a RawProvider opts THAT ONE function into the typed lift/lower path while
// every other name — and every cell that never calls ProvideRaw — keeps the
// existing msgpack behaviour byte-for-byte. Overwrites any prior raw
// registration with the same name.
func ProvideRaw(name string, handler RawProvider) {
	rawProvidersMu.Lock()
	rawProviders[name] = handler
	rawProvidersMu.Unlock()
}

// dispatchOnCall is the body of the pulp_on_call export. The host
// invokes this when a sibling cell calls Call(thisCell, name, ...).
// Returns 0 on success, nonzero to signal an error to the caller.
func dispatchOnCall(namePtr, nameLen, argsPtr, argsLen, respPtrOut, respLenOut uint32) uint32 {
	if nameLen == 0 {
		return 1
	}
	name := string(unsafe.Slice((*byte)(unsafe.Pointer(uintptr(namePtr))), nameLen))

	// Canonical-ABI (witcell) path — checked FIRST, additively. If a
	// RawProvider is registered for this name, hand it the raw pointers: it
	// lifts the request from linear memory, runs the typed handler, lowers the
	// result as a pinned pointer tree, and writes (respPtr, respLen) to the
	// out-params itself. The host then tree-frees via pulp_post_return. If no
	// raw provider is registered (the case for every legacy msgpack cell), we
	// fall through to the unchanged msgpack path below.
	rawProvidersMu.RLock()
	raw, rawOK := rawProviders[name]
	rawProvidersMu.RUnlock()
	if rawOK {
		return raw(argsPtr, argsLen, respPtrOut, respLenOut)
	}

	var args []byte
	if argsLen > 0 {
		args = unsafe.Slice((*byte)(unsafe.Pointer(uintptr(argsPtr))), argsLen)
	}

	providersMu.RLock()
	handler, ok := providers[name]
	providersMu.RUnlock()
	if !ok {
		return 10 // "no provider for name"
	}

	resp, err := handler(args)
	if err != nil {
		log.Printf("[pulp] provider %q failed: %v", name, err)
		return 11 // "provider returned error"
	}
	if len(resp) == 0 {
		writeOutPtr(respPtrOut, 0)
		writeOutLen(respLenOut, 0)
		return 0
	}

	// Copy the response into WASM memory that the host can reach and
	// register it with the cell's alloc table so Go's GC doesn't free
	// it before the host reads. Same failure mode we fixed in
	// lifecycle.go's pulpAlloc — without the table, `buf` goes out of
	// scope when dispatchOnCall returns, the host reads stale memory,
	// and sibling-call responses silently corrupt. Host calls pulp_free
	// after copying the bytes (see Fiber engine helpers) to release.
	buf := make([]byte, len(resp))
	copy(buf, resp)
	ptr := uint32(uintptr(unsafe.Pointer(&buf[0])))
	allocMu.Lock()
	allocTable[ptr] = buf
	allocMu.Unlock()
	writeOutPtr(respPtrOut, ptr)
	writeOutLen(respLenOut, uint32(len(buf)))
	return 0
}

// writeOutPtr / writeOutLen write a uint32 into cell linear memory
// at the address encoded in outAddr. outAddr is itself a cell-memory
// offset (the host writes caller-provided out-param addresses here).
// We use unsafe directly rather than the wazero memory API because
// we're on the cell side.
func writeOutPtr(outAddr, val uint32) {
	*(*uint32)(unsafe.Pointer(uintptr(outAddr))) = val
}
func writeOutLen(outAddr, val uint32) { writeOutPtr(outAddr, val) }

func siblingCodeError(c uint32) error {
	switch c {
	case 1:
		return fmt.Errorf("pulp.Call: empty target or func name")
	case 2:
		return fmt.Errorf("pulp.Call: host memory read failed")
	case 4:
		return fmt.Errorf("pulp.Call: target cell returned error or trapped")
	case 7:
		return fmt.Errorf("pulp.Call: host pulp_alloc failed")
	case 8:
		return fmt.Errorf("pulp.Call: host memory write failed")
	case 11:
		return fmt.Errorf("pulp.Call: not authorized — declare target in consumes or depends_on")
	case 99:
		return fmt.Errorf("pulp.Call: sibling-call capability not bound in this host")
	default:
		return fmt.Errorf("pulp.Call: host error code %d", c)
	}
}

//go:wasmexport pulp_on_call
func pulpOnCall(namePtr, nameLen, argsPtr, argsLen, respPtrOut, respLenOut uint32) uint32 {
	return dispatchOnCall(namePtr, nameLen, argsPtr, argsLen, respPtrOut, respLenOut)
}
