package pulp

import (
	"fmt"
	"runtime"
	"unsafe"
)

// hostPulpAppCall is available only when Pulp starts a multi-application host
// (`pulp -host`). The host checks the caller application's manifest-declared
// dependency before routing the call to the exact target app instance/cell
// provider. It deliberately accepts no default application or instance.
//

// AppCall invokes provider on one explicitly named cell in one explicitly
// named application instance. The result is opaque bytes. This is distinct
// from Call: a local cell-to-cell capability is never silently widened into a
// cross-application one.
func AppCall(app, instance, cell, provider string, args []byte) ([]byte, error) {
	if app == "" || instance == "" || cell == "" || provider == "" {
		return nil, fmt.Errorf("pulp.AppCall: app, instance, cell, and provider are required")
	}
	appBytes := []byte(app)
	instanceBytes := []byte(instance)
	cellBytes := []byte(cell)
	providerBytes := []byte(provider)

	var argsPtr, argsLen uint32
	if len(args) > 0 {
		argsPtr = uint32(uintptr(unsafe.Pointer(&args[0])))
		argsLen = uint32(len(args))
	}

	var respPtr, respLen uint32
	code := hostPulpAppCall(
		uint32(uintptr(unsafe.Pointer(&appBytes[0]))), uint32(len(appBytes)),
		uint32(uintptr(unsafe.Pointer(&instanceBytes[0]))), uint32(len(instanceBytes)),
		uint32(uintptr(unsafe.Pointer(&cellBytes[0]))), uint32(len(cellBytes)),
		uint32(uintptr(unsafe.Pointer(&providerBytes[0]))), uint32(len(providerBytes)),
		argsPtr, argsLen,
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(appBytes)
	runtime.KeepAlive(instanceBytes)
	runtime.KeepAlive(cellBytes)
	runtime.KeepAlive(providerBytes)
	runtime.KeepAlive(args)
	if code != 0 {
		return nil, appCallCodeError(code)
	}
	if respLen == 0 {
		return nil, nil
	}
	resp := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	out := append([]byte(nil), resp...)
	pulpFree(respPtr, respLen)
	return out, nil
}

func appCallCodeError(code uint32) error {
	switch code {
	case 1:
		return fmt.Errorf("pulp.AppCall: empty required identifier")
	case 2:
		return fmt.Errorf("pulp.AppCall: guest memory read failed")
	case 4:
		return fmt.Errorf("pulp.AppCall: target unavailable or provider call failed")
	case 7:
		return fmt.Errorf("pulp.AppCall: host response allocation failed")
	case 8:
		return fmt.Errorf("pulp.AppCall: host response write failed")
	case 11:
		return fmt.Errorf("pulp.AppCall: not authorized — declare target application in depends_on")
	case 99:
		return fmt.Errorf("pulp.AppCall: host recovered from panic")
	default:
		return fmt.Errorf("pulp.AppCall: host error code %d", code)
	}
}
