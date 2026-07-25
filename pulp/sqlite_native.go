//go:build !wasip1

package pulp

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostSQLiteExec(qPtr, qLen, pPtr, pLen, resPtrOut, resLenOut uint32) uint32 { return 99 }

func hostSQLiteQuery(qPtr, qLen, pPtr, pLen, rowsPtrOut, rowsLenOut uint32) uint32 { return 99 }
