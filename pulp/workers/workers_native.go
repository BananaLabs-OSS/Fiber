//go:build !wasip1

package workers

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostSubmit(reqPtr, reqLen uint32) uint32 { return 99 }

func hostSubmitFire(reqPtr, reqLen uint32) uint32 { return 99 }

func hostResult(taskID, resultPtrOut, resultLenOut uint32) uint32 { return 99 }

func hostCancel(taskID uint32) uint32 { return 99 }

func hostPending() uint32 { return 99 }
