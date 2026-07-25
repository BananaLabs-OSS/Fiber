//go:build wasip1

package pulp

//go:wasmimport pulp pulp_app_call_v1
func hostPulpAppCall(
	appPtr, appLen,
	instancePtr, instanceLen,
	cellPtr, cellLen,
	providerPtr, providerLen,
	argsPtr, argsLen,
	respPtrOut, respLenOut uint32,
) uint32
