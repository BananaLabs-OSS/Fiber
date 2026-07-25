//go:build !wasip1

package docker

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostList(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostGet(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostDestroy(reqPtr, reqLen uint32) uint32 { return 99 }

func hostRestart(reqPtr, reqLen uint32) uint32 { return 99 }

func hostExec(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostLogs(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostStats(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostFilesRead(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostFilesWrite(reqPtr, reqLen uint32) uint32 { return 99 }

func hostFilesDelete(reqPtr, reqLen uint32) uint32 { return 99 }

func hostEventsPoll(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostStatsAll(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostBuild(reqPtr, reqLen uint32) uint32 { return 99 }

func hostBuildStatus(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }
