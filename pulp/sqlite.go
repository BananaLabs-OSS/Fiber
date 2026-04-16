package pulp

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// SQLite groups host-import wrappers for the storage.sqlite capability.
// The plugin must declare "storage.sqlite" in its manifest.
var SQLite = sqliteAPI{}

type sqliteAPI struct{}

//go:wasmimport pulp sqlite_exec
func hostSQLiteExec(qPtr, qLen, pPtr, pLen uint32) uint32

//go:wasmimport pulp sqlite_query
func hostSQLiteQuery(qPtr, qLen, pPtr, pLen, rowsPtrOut, rowsLenOut uint32) uint32

// Exec runs a statement that does not return rows (INSERT, UPDATE,
// DELETE, CREATE TABLE, etc.). args are positional parameters matching
// SQLite ? placeholders.
func (sqliteAPI) Exec(query string, args ...any) error {
	q := []byte(query)
	var pPtr, pLen uint32
	var paramBytes []byte
	if len(args) > 0 {
		var err error
		paramBytes, err = msgpack.Marshal(args)
		if err != nil {
			return fmt.Errorf("encode args: %w", err)
		}
		pPtr = uint32(uintptr(unsafe.Pointer(&paramBytes[0])))
		pLen = uint32(len(paramBytes))
	}
	code := hostSQLiteExec(
		uint32(uintptr(unsafe.Pointer(&q[0]))),
		uint32(len(q)),
		pPtr,
		pLen,
	)
	runtime.KeepAlive(q)
	runtime.KeepAlive(paramBytes)
	if code != 0 {
		return fmt.Errorf("sqlite_exec host code %d", code)
	}
	return nil
}

// Query runs a SELECT statement and returns rows as [][]any — outer
// slice is rows, inner slice is column values in declaration order.
// Callers usually know the column order from their own query.
func (sqliteAPI) Query(query string, args ...any) ([][]any, error) {
	q := []byte(query)
	var pPtr, pLen uint32
	var paramBytes []byte
	if len(args) > 0 {
		var err error
		paramBytes, err = msgpack.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("encode args: %w", err)
		}
		pPtr = uint32(uintptr(unsafe.Pointer(&paramBytes[0])))
		pLen = uint32(len(paramBytes))
	}
	var rowsPtr, rowsLen uint32
	code := hostSQLiteQuery(
		uint32(uintptr(unsafe.Pointer(&q[0]))),
		uint32(len(q)),
		pPtr,
		pLen,
		uint32(uintptr(unsafe.Pointer(&rowsPtr))),
		uint32(uintptr(unsafe.Pointer(&rowsLen))),
	)
	runtime.KeepAlive(q)
	runtime.KeepAlive(paramBytes)
	if code != 0 {
		return nil, fmt.Errorf("sqlite_query host code %d", code)
	}
	if rowsLen == 0 {
		return nil, nil
	}
	rowBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(rowsPtr))), rowsLen)
	var rows [][]any
	if err := msgpack.Unmarshal(rowBytes, &rows); err != nil {
		return nil, fmt.Errorf("decode rows: %w", err)
	}
	return rows, nil
}
