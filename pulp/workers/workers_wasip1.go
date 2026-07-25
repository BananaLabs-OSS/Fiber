//go:build wasip1

package workers

//go:wasmimport pulp workers_submit
func hostSubmit(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp workers_submit_fire
func hostSubmitFire(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp workers_result
func hostResult(taskID, resultPtrOut, resultLenOut uint32) uint32

//go:wasmimport pulp workers_cancel
func hostCancel(taskID uint32) uint32

//go:wasmimport pulp workers_pending
func hostPending() uint32
