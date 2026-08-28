// Package sql is a database/sql driver backed by Pulp's storage.sqlite
// capability. Importing this package (for side effects) registers the
// driver under the name "pulp", so cells can use Go's standard
// database/sql surface — and any library that accepts a *sql.DB, such
// as Bun, GORM, sqlx — against a Pulp-hosted SQLite database.
//
// Typical usage:
//
//	import (
//		"database/sql"
//		_ "github.com/BananaLabs-OSS/Fiber/pulp/sql"
//	)
//
//	db, _ := sql.Open("pulp", "")
//	// db is a *sql.DB backed by pulp.SQLite.Exec / pulp.SQLite.Query.
//
//	// With Bun:
//	bdb := bun.NewDB(db, sqlitedialect.New())
//
// The cell manifest must declare "storage.sqlite" in capabilities.
// The DSN is ignored — the host assigns the database path based on
// the cell name and the host's -storage-root flag.
package sql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"

	"github.com/BananaLabs-OSS/Fiber/pulp"
)

// DriverName is the name this driver registers under.
const DriverName = "pulp"

func init() {
	sql.Register(DriverName, &Driver{})
}

// Driver implements database/sql/driver.Driver. Open ignores its
// argument — there is exactly one database per cell, assigned by
// the host, and the cell does not choose its path.
type Driver struct{}

// Open returns a new Conn. The dsn argument is ignored; pass any
// string (convention: "").
func (*Driver) Open(_ string) (driver.Conn, error) {
	return &Conn{}, nil
}

// Conn represents the cell's single logical connection to the host
// SQLite database. Connection pooling on the Go side is a no-op
// because the host pins its own pool to one connection; multiple
// Conn values share the same underlying database transparently.
type Conn struct {
	// inTx tracks whether a BEGIN has been issued but not yet
	// COMMIT/ROLLBACK. Nested transactions are not supported —
	// SQLite doesn't offer them natively either.
	inTx bool
}

// Prepare wraps a SQL string in a Stmt. We do no parsing or parameter
// counting here — SQLite tolerates extra or missing parameters at
// execute time, and database/sql handles the argument plumbing.
func (c *Conn) Prepare(query string) (driver.Stmt, error) {
	return &Stmt{conn: c, query: query}, nil
}

// Close is a no-op: the host owns the real connection lifecycle.
func (*Conn) Close() error { return nil }

// Begin starts a transaction by issuing BEGIN. Because the host pins
// MaxOpenConns to 1, all subsequent statements on this cell land
// on the same connection, so BEGIN/COMMIT work as expected.
func (c *Conn) Begin() (driver.Tx, error) {
	if c.inTx {
		return nil, fmt.Errorf("nested transaction not supported")
	}
	if _, err := pulp.SQLite.Exec("BEGIN"); err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	c.inTx = true
	return &Tx{conn: c}, nil
}

// BeginTx is the context-aware variant. Opts are accepted but most
// are ignored — SQLite offers limited isolation tuning and the host
// does not expose it.
func (c *Conn) BeginTx(_ context.Context, _ driver.TxOptions) (driver.Tx, error) {
	return c.Begin()
}

// Ping is a no-op; the host is always available during cell lifetime.
func (*Conn) Ping(_ context.Context) error { return nil }

// Stmt is a prepared statement. It holds the query text and a back
// reference to the connection — arguments arrive on Exec/Query.
type Stmt struct {
	conn  *Conn
	query string
}

// Close is a no-op — there is no host-side statement handle.
func (*Stmt) Close() error { return nil }

// NumInput returns -1 meaning "unknown" — lets database/sql pass
// whatever argument count the caller provided without precheck.
func (*Stmt) NumInput() int { return -1 }

// Exec runs a non-query statement. Args are converted to []any for
// delivery to the host; the host's ExecResult (rows_affected +
// last_insert_id) is translated into a driver.Result so database/sql
// callers like Bun can check update counts and insert IDs.
func (s *Stmt) Exec(args []driver.Value) (driver.Result, error) {
	r, err := pulp.SQLite.Exec(s.query, driverValuesToAny(args)...)
	if err != nil {
		return nil, err
	}
	return execResult{rowsAffected: r.RowsAffected, lastInsertID: r.LastInsertID}, nil
}

// ExecContext is the context-aware variant. The host call is
// synchronous through WASM so we cannot cancel mid-statement, but we
// honor already-cancelled contexts before entering the host and after
// it returns.
func (s *Stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	res, err := s.Exec(namedValuesToDriverValues(args))
	if err != nil {
		return res, err
	}
	return res, ctx.Err()
}

// Query runs a SELECT statement and returns a driver.Rows.
func (s *Stmt) Query(args []driver.Value) (driver.Rows, error) {
	result, err := pulp.SQLite.Query(s.query, driverValuesToAny(args)...)
	if err != nil {
		return nil, err
	}
	return &Rows{columns: result.Columns, rows: result.Rows}, nil
}

// QueryContext is the context-aware variant. Like ExecContext, we
// honor ctx.Err() before and after the host call.
func (s *Stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows, err := s.Query(namedValuesToDriverValues(args))
	if err != nil {
		return rows, err
	}
	if cerr := ctx.Err(); cerr != nil {
		if rows != nil {
			_ = rows.Close()
		}
		return nil, cerr
	}
	return rows, nil
}

// Rows iterates a materialized query result.
type Rows struct {
	columns []string
	rows    [][]any
	idx     int
}

// Columns returns the column names in the result.
func (r *Rows) Columns() []string { return r.columns }

// Close releases the result — no resources to free since rows are
// already fully materialized in memory.
func (r *Rows) Close() error {
	r.rows = nil
	return nil
}

// Next advances to the next row, copying values into dest.
func (r *Rows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	row := r.rows[r.idx]
	r.idx++
	for i := range dest {
		if i >= len(row) {
			dest[i] = nil
			continue
		}
		dest[i] = toDriverValue(row[i])
	}
	return nil
}

// Tx is a live transaction on a Conn. Commit or Rollback exactly once.
type Tx struct {
	conn *Conn
}

// Commit closes the transaction with COMMIT.
func (t *Tx) Commit() error {
	t.conn.inTx = false
	if _, err := pulp.SQLite.Exec("COMMIT"); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Rollback closes the transaction with ROLLBACK.
func (t *Tx) Rollback() error {
	t.conn.inTx = false
	if _, err := pulp.SQLite.Exec("ROLLBACK"); err != nil {
		return fmt.Errorf("rollback: %w", err)
	}
	return nil
}

// execResult carries the rows-affected and last-insert-id fields that
// the host's sqlite_exec reports.
type execResult struct {
	rowsAffected int64
	lastInsertID int64
}

func (r execResult) LastInsertId() (int64, error) { return r.lastInsertID, nil }
func (r execResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }

// driverValuesToAny adapts []driver.Value to []any for host delivery.
func driverValuesToAny(in []driver.Value) []any {
	if len(in) == 0 {
		return nil
	}
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

// namedValuesToDriverValues strips names — SQLite positional bindings
// do not use them. If callers pass named parameters we treat them as
// positional in order received.
func namedValuesToDriverValues(in []driver.NamedValue) []driver.Value {
	if len(in) == 0 {
		return nil
	}
	out := make([]driver.Value, len(in))
	for i, v := range in {
		out[i] = v.Value
	}
	return out
}

// toDriverValue coerces a decoded msgpack value into a type
// database/sql accepts. msgpack/v5 decodes numbers as int64/uint64 or
// float64, strings as string, byte slices as []byte, nil as nil, and
// timestamp-ext values as time.Time — all of which are already valid
// driver.Value types or legal to hand off.
//
// uint64 values above MaxInt64 are returned as decimal strings so the
// magnitude is preserved (int64 would silently overflow). Scanners
// targeting an integer field will error rather than read a wrong value.
func toDriverValue(v any) driver.Value {
	switch x := v.(type) {
	case nil, bool, int64, float64, []byte, string, time.Time:
		return x
	case uint64:
		if x <= math.MaxInt64 {
			return int64(x)
		}
		return strconv.FormatUint(x, 10)
	case uint:
		if uint64(x) <= math.MaxInt64 {
			return int64(x)
		}
		return strconv.FormatUint(uint64(x), 10)
	case uint32:
		return int64(x)
	case uint16:
		return int64(x)
	case uint8:
		return int64(x)
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case int16:
		return int64(x)
	case int8:
		return int64(x)
	case float32:
		return float64(x)
	default:
		return fmt.Sprintf("%v", x)
	}
}
