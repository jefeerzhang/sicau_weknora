package database

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	migratedatabase "github.com/golang-migrate/migrate/v4/database"
)

// SQLite lacks ADD COLUMN IF NOT EXISTS. Only the re-numbered teaching
// migrations need conditional column additions; other migrations retain the
// upstream driver's strict execution and transaction behaviour.
type teachingSQLiteDriver struct {
	migratedatabase.Driver
	db *sql.DB
}

func (d *teachingSQLiteDriver) Run(migration io.Reader) error {
	query, err := io.ReadAll(migration)
	if err != nil {
		return err
	}
	text := string(query)
	if strings.Contains(text, "sicau-v1 ticket 04: workspace default agent id") ||
		strings.Contains(text, "SQLite: platform identity flags for SuperAdmin bootstrap") {
		text, err = omitExistingSQLiteColumns(context.Background(), d.db, text)
		if err != nil {
			return err
		}
	}
	return d.Driver.Run(strings.NewReader(text))
}

type sqliteSchemaReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

var sqliteAddColumn = regexp.MustCompile(`(?i)ALTER TABLE ([a-z_]+) ADD COLUMN ([a-z_]+) [^;]+;`)

func sqliteHasColumn(ctx context.Context, db sqliteSchemaReader, table, column string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info(?) WHERE name = ?", table, column).Scan(&count)
	return count > 0, err
}

func omitExistingSQLiteColumns(ctx context.Context, db sqliteSchemaReader, query string) (string, error) {
	var schemaErr error
	query = sqliteAddColumn.ReplaceAllStringFunc(query, func(statement string) string {
		if schemaErr != nil {
			return statement
		}
		parts := sqliteAddColumn.FindStringSubmatch(statement)
		var exists bool
		exists, schemaErr = sqliteHasColumn(ctx, db, parts[1], parts[2])
		if exists {
			return ""
		}
		return statement
	})
	return query, schemaErr
}

// Old teaching SQLite versions 13–18 carried different SQL. Replay the
// upstream migrations hidden behind those slots without rewriting the version
// ledger. All bridge writes commit together, so a failure is safe to retry.
func replaySQLiteTeachingCollisions(ctx context.Context, db *sql.DB, version uint) error {
	if version < 13 {
		return nil
	}
	teaching, err := sqliteHasColumn(ctx, db, "users", "is_teacher")
	if err != nil || !teaching {
		return err
	}
	enabled, err := sqliteHasColumn(ctx, db, "mcp_tool_approvals", "enabled")
	if err != nil || enabled {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for v := uint(13); v <= 18 && v <= version; v++ {
		paths, err := filepath.Glob(fmt.Sprintf("migrations/sqlite/%06d_*.up.sql", v))
		if err != nil {
			return err
		}
		if len(paths) != 1 {
			return fmt.Errorf("SQLite teaching bridge requires exactly one upstream migration %06d", v)
		}
		contents, err := os.ReadFile(paths[0])
		if err != nil {
			return err
		}
		query, err := omitExistingSQLiteColumns(ctx, tx, string(contents))
		if err != nil {
			return err
		}
		// Preserve schema that may already have been applied by an operator.
		query = strings.ReplaceAll(query, "CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ")
		query = strings.ReplaceAll(query, "CREATE INDEX ", "CREATE INDEX IF NOT EXISTS ")
		query = strings.ReplaceAll(query, "IF NOT EXISTS IF NOT EXISTS", "IF NOT EXISTS")
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("SQLite teaching bridge %06d: %w", v, err)
		}
	}
	return tx.Commit()
}
