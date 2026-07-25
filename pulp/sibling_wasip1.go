//go:build wasip1

package pulp

//go:wasmimport pulp pulp_call
func hostPulpCall(
	targetPtr, targetLen,
	namePtr, nameLen,
	argsPtr, argsLen,
	respPtrOut, respLenOut uint32,
) uint32
