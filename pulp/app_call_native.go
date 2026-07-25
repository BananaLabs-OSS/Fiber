//go:build !wasip1

package pulp

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostPulpAppCall(
	appPtr, appLen,
	instancePtr, instanceLen,
	cellPtr, cellLen,
	providerPtr, providerLen,
	argsPtr, argsLen,
	respPtrOut, respLenOut uint32,
) uint32 {
	return 99
}
