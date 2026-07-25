//go:build wasip1

package pulp

//go:wasmimport pulp toolchain_install
func hostToolchainInstall(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp toolchain_status
func hostToolchainStatus(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32
