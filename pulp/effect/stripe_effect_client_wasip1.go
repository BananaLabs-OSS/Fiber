//go:build wasip1

package effect

import (
	"runtime"
	"unsafe"

	"github.com/BananaLabs-OSS/Fiber/pulp"
)

//go:wasmimport pulp stripe_effect_execute
func hostStripeEffectExecute(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

var stripeEffectExecuteWire = hostExecuteStripeEffectWire

func hostExecuteStripeEffectWire(request []byte) ([]byte, uint32) {
	var responsePtr, responseLen uint32
	code := hostStripeEffectExecute(
		uint32(uintptr(unsafe.Pointer(&request[0]))),
		uint32(len(request)),
		uint32(uintptr(unsafe.Pointer(&responsePtr))),
		uint32(uintptr(unsafe.Pointer(&responseLen))),
	)
	runtime.KeepAlive(request)
	if code != 0 || responseLen == 0 || responsePtr == 0 {
		return nil, code
	}
	response := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(responsePtr))), responseLen)
	wire := append([]byte(nil), response...)
	pulp.ReleaseHostAlloc(responsePtr, responseLen)
	return wire, 0
}
