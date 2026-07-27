//go:build wasip1

package effect

import (
	"runtime"
	"unsafe"

	"github.com/BananaLabs-OSS/Fiber/pulp"
)

//go:wasmimport pulp server_mutation_execute_v4
func hostServerMutationExecuteV4(
	requestPointer, requestLength, responsePointerOut, responseLengthOut uint32,
) uint32

var serverMutationHostExecuteWire = func(request []byte) ([]byte, uint32) {
	var responsePointer, responseLength uint32
	code := hostServerMutationExecuteV4(
		uint32(uintptr(unsafe.Pointer(&request[0]))),
		uint32(len(request)),
		uint32(uintptr(unsafe.Pointer(&responsePointer))),
		uint32(uintptr(unsafe.Pointer(&responseLength))),
	)
	runtime.KeepAlive(request)
	if code != 0 || responsePointer == 0 || responseLength == 0 {
		return nil, code
	}
	response := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(responsePointer))), responseLength)
	wire := append([]byte(nil), response...)
	pulp.ReleaseHostAlloc(responsePointer, responseLength)
	return wire, 0
}
