package pulp

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// Sentinel errors surfaced by SQLite.Exec and SQLite.Query. Cells can
// branch on these with errors.Is — the host maps sqlite error codes to
// fixed host-return codes which map back to these values here.
var (
	ErrSQLiteBusy       = errors.New("pulp/sqlite: database busy or locked")
	ErrSQLiteConstraint = errors.New("pulp/sqlite: constraint violation")
	ErrSQLiteReadonly   = errors.New("pulp/sqlite: database is readonly")
)

// classifySQLiteError maps the host return code (5/12/13/14) to a
// typed sentinel, wrapping the host-supplied detail when present.
func classifySQLiteError(code uint32, detail string) error {
	var base error
	switch code {
	case 12:
		base = ErrSQLiteBusy
	case 13:
		base = ErrSQLiteConstraint
	case 14:
		base = ErrSQLiteReadonly
	default:
		if detail != "" {
			return fmt.Errorf("pulp/sqlite: %s", detail)
		}
		return fmt.Errorf("pulp/sqlite: host code %d", code)
	}
	if detail != "" {
		return fmt.Errorf("%w: %s", base, detail)
	}
	return base
}

// SQLite groups host-import wrappers for the storage.sqlite capability.
// The cell must declare "storage.sqlite" in its manifest.
var SQLite = sqliteAPI{}

type sqliteAPI struct{}

//go:wasmimport pulp sqlite_exec
func hostSQLiteExec(qPtr, qLen, pPtr, pLen, resPtrOut, resLenOut uint32) uint32

//go:wasmimport pulp sqlite_query
func hostSQLiteQuery(qPtr, qLen, pPtr, pLen, rowsPtrOut, rowsLenOut uint32) uint32

// ExecResult is what Exec returns: rows affected + last insert row ID.
// Matches the host's msgpack shape. Error is populated when the host
// returns a non-zero code; callers see it as a returned error, not a
// field.
type ExecResult struct {
	RowsAffected int64  `msgpack:"rows_affected"`
	LastInsertID int64  `msgpack:"last_insert_id"`
	Error        string `msgpack:"error,omitempty"`
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
	if code == 99 {
		return ExecResult{}, ErrCapabilityUnavailable
	}
	var result ExecResult
	if resLen > 0 {
		resBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(resPtr))), resLen)
		buf := make([]byte, resLen)
		copy(buf, resBytes)
		releaseHostAlloc(resPtr, resLen)
		if err := msgpack.Unmarshal(buf, &result); err != nil {
			return ExecResult{}, fmt.Errorf("decode exec result: %w", err)
		}
	}
	if code != 0 {
		return ExecResult{}, classifySQLiteError(code, result.Error)
	}
	return result, nil
}

// QueryResult is what Query returns — column names plus rows, where
// each row's values are in Columns order. Error carries host-side
// error detail; callers see it as a returned error, not a field.
type QueryResult struct {
	Columns []string `msgpack:"columns"`
	Rows    [][]any  `msgpack:"rows"`
	Error   string   `msgpack:"error,omitempty"`
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
	if code == 99 {
		return QueryResult{}, ErrCapabilityUnavailable
	}
	var result QueryResult
	if rowsLen > 0 {
		rowBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(rowsPtr))), rowsLen)
		buf := make([]byte, rowsLen)
		copy(buf, rowBytes)
		releaseHostAlloc(rowsPtr, rowsLen)
		if err := msgpack.Unmarshal(buf, &result); err != nil {
			return QueryResult{}, fmt.Errorf("decode query result: %w", err)
		}
	}
	if code != 0 {
		return QueryResult{}, classifySQLiteError(code, result.Error)
	}
	return result, nil
}
