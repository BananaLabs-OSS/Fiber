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
func hostSQLiteExec(qPtr, qLen, pPtr, pLen, resPtrOut, resLenOut uint32) uint32

//go:wasmimport pulp sqlite_query
func hostSQLiteQuery(qPtr, qLen, pPtr, pLen, rowsPtrOut, rowsLenOut uint32) uint32

// ExecResult is what Exec returns: rows affected + last insert row ID.
// Matches the host's msgpack shape.
type ExecResult struct {
	RowsAffected int64 `msgpack:"rows_affected"`
	LastInsertID int64 `msgpack:"last_insert_id"`
}

// Exec runs a statement that does not return rows (INSERT, UPDATE,
// DELETE, CREATE TABLE, etc.). args are positional parameters matching
// SQLite ? placeholders. Returns the rows-affected count and the
// last-insert row ID the host produced.
func (sqliteAPI) Exec(query string, args ...any) (ExecResult, error) {
	q := []byte(query)
	var pPtr, pLen uint32
	var paramBytes []byte
	if len(args) > 0 {
		var err error
		paramBytes, err = msgpack.Marshal(args)
		if err != nil {
			return ExecResult{}, fmt.Errorf("encode args: %w", err)
		}
		pPtr = uint32(uintptr(unsafe.Pointer(&paramBytes[0])))
		pLen = uint32(len(paramBytes))
	}
	var resPtr, resLen uint32
	code := hostSQLiteExec(
		uint32(uintptr(unsafe.Pointer(&q[0]))),
		uint32(len(q)),
		pPtr,
		pLen,
		uint32(uintptr(unsafe.Pointer(&resPtr))),
		uint32(uintptr(unsafe.Pointer(&resLen))),
	)
	runtime.KeepAlive(q)
	runtime.KeepAlive(paramBytes)
	if code != 0 {
		return ExecResult{}, fmt.Errorf("sqlite_exec host code %d", code)
	}
	if resLen == 0 {
		return ExecResult{}, nil
	}
	resBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(resPtr))), resLen)
	var result ExecResult
	if err := msgpack.Unmarshal(resBytes, &result); err != nil {
		return ExecResult{}, fmt.Errorf("decode exec result: %w", err)
	}
	return result, nil
}

// QueryResult is what Query returns — column names plus rows, where
// each row's values are in Columns order.
type QueryResult struct {
	Columns []string `msgpack:"columns"`
	Rows    [][]any  `msgpack:"rows"`
}

// Query runs a SELECT statement and returns the column names plus
// rows. Use this for ad-hoc queries; for ORM integration use the
// pulp/sql driver which wraps Query in Go's database/sql surface.
func (sqliteAPI) Query(query string, args ...any) (QueryResult, error) {
	q := []byte(query)
	var pPtr, pLen uint32
	var paramBytes []byte
	if len(args) > 0 {
		var err error
		paramBytes, err = msgpack.Marshal(args)
		if err != nil {
			return QueryResult{}, fmt.Errorf("encode args: %w", err)
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
		return QueryResult{}, fmt.Errorf("sqlite_query host code %d", code)
	}
	if rowsLen == 0 {
		return QueryResult{}, nil
	}
	rowBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(rowsPtr))), rowsLen)
	var result QueryResult
	if err := msgpack.Unmarshal(rowBytes, &result); err != nil {
		return QueryResult{}, fmt.Errorf("decode query result: %w", err)
	}
	return result, nil
}
