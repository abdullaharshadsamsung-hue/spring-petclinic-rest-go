// Package schema embeds the PetClinic DDL and seed scripts for every
// supported dialect and applies them at startup (the equivalent of Spring's
// spring.sql.init.mode=always).
package schema

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

// Supported dialects.
const (
	SQLite   = "sqlite"
	Postgres = "postgres"
)

//go:embed sqlite/*.sql postgres/*.sql
var scripts embed.FS

// Apply runs every script of dialect (0001_schema.sql, 0002_seed.sql, ...)
// in lexical order inside a single transaction. All scripts are idempotent,
// so Apply is safe to run on every start.
func Apply(ctx context.Context, db *sql.DB, dialect string) (err error) {
	names, err := fs.Glob(scripts, dialect+"/*.sql")
	if err != nil {
		return fmt.Errorf("schema: list %s scripts: %w", dialect, err)
	}
	if len(names) == 0 {
		return fmt.Errorf("schema: unsupported dialect %q", dialect)
	}
	sort.Strings(names)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("schema: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	for _, name := range names {
		body, rerr := scripts.ReadFile(name)
		if rerr != nil {
			err = fmt.Errorf("schema: read %s: %w", name, rerr)
			return err
		}
		if _, err = tx.ExecContext(ctx, string(body)); err != nil {
			err = fmt.Errorf("schema: apply %s: %w", name, err)
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("schema: commit: %w", err)
	}
	return nil
}
