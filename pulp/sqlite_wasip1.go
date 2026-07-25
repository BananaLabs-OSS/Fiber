//go:build wasip1

package pulp

//go:wasmimport pulp sqlite_exec
func hostSQLiteExec(qPtr, qLen, pPtr, pLen, resPtrOut, resLenOut uint32) uint32

//go:wasmimport pulp sqlite_query
func hostSQLiteQuery(qPtr, qLen, pPtr, pLen, rowsPtrOut, rowsLenOut uint32) uint32
