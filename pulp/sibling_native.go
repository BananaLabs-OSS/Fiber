//go:build !wasip1

package pulp

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostPulpCall(
	targetPtr, targetLen,
	namePtr, nameLen,
	argsPtr, argsLen,
	respPtrOut, respLenOut uint32,
) uint32 {
	return 99
}
