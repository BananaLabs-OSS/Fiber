//go:build !wasip1

package pulp

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostFSRead(pathPtr, pathLen, dataPtrOut, dataLenOut uint32) uint32 { return 99 }

func hostFSWrite(pathPtr, pathLen, dataPtr, dataLen, reqPtr, reqLen uint32) uint32 { return 99 }

func hostFSDelete(pathPtr, pathLen uint32) uint32 { return 99 }

func hostFSList(pathPtr, pathLen, dataPtrOut, dataLenOut uint32) uint32 { return 99 }

func hostFSStat(reqPtr, reqLen, dataPtrOut, dataLenOut uint32) uint32 { return 99 }

func hostFSRename(reqPtr, reqLen uint32) uint32 { return 99 }

func hostFSRemoveAll(pathPtr, pathLen uint32) uint32 { return 99 }

func hostFSMkdirAll(reqPtr, reqLen uint32) uint32 { return 99 }

func hostFSChmod(reqPtr, reqLen uint32) uint32 { return 99 }

func hostFSCreateTemp(reqPtr, reqLen, dataPtrOut, dataLenOut uint32) uint32 { return 99 }

func hostFSMkdirTemp(reqPtr, reqLen, dataPtrOut, dataLenOut uint32) uint32 { return 99 }
