//go:build !wasip1

package pulp

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostProcessRun(reqPtr, reqLen uint32) uint32 { return 99 }

func hostProcessResult(taskID, outPtrOut, outLenOut uint32) uint32 { return 99 }

func hostProcessCancel(taskID uint32) uint32 { return 99 }

func hostProcessPending() uint32 { return 99 }
