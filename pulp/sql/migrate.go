package sql

import (
	"context"
	"database/sql"
	"fmt"
)

// Index describes a SQL index to create during Migrate. Query is the
// full CREATE INDEX statement (typically with IF NOT EXISTS).
type Index struct {
	Name  string
	Query string
}

// Migrate executes each DDL statement in tables and each Index query
// against db. Statements are expected to be idempotent (CREATE TABLE
// IF NOT EXISTS, CREATE INDEX IF NOT EXISTS) so Migrate can safely run
// on every cell start.
//
// This is the cell-side equivalent of Potassium's database.Migrate,
// which takes bun models directly. Cells using Bun can call
// Migrate(ctx, db.DB, ddl, indexes) where ddl is rendered from
// bun.NewCreateTable().Model(m).IfNotExists() — see MigrateBun for a
// helper that wraps that.
func Migrate(ctx context.Context, db *sql.DB, tables []string, indexes []Index) error {
	for _, ddl := range tables {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("table DDL: %w", err)
		}
	}
	for _, idx := range indexes {
		if _, err := db.ExecContext(ctx, idx.Query); err != nil {
			return fmt.Errorf("index %s: %w", idx.Name, err)
		}
	}
	return nil
}
