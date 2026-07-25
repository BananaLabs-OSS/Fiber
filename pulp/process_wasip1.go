//go:build wasip1

package pulp

//go:wasmimport pulp process_run
func hostProcessRun(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp process_result
func hostProcessResult(taskID, outPtrOut, outLenOut uint32) uint32

//go:wasmimport pulp process_cancel
func hostProcessCancel(taskID uint32) uint32

//go:wasmimport pulp process_pending
func hostProcessPending() uint32
