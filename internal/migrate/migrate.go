// Package migrate applies embedded SQL migrations. It is process
// infrastructure, not a domain service (ADR-0002, ADR-0005). Production
// code uses database/sql only. Drivers are blank-imported in cmd/veylqo,
// which calls Apply on start (VYQ-38). Tests may register a driver.
package migrate

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"time"
)

// Dialect selects a migration directory. Dialects are not forced into one
// file (ADR-0005).
type Dialect string

const (
	// Postgres reads migrations/postgres. Apply takes a session advisory
	// lock so two processes do not migrate at once.
	Postgres Dialect = "postgres"

	// SQLite reads migrations/sqlite. There is no advisory lock. One
	// writer is the lock. SQLite does not replace Postgres under load.
	SQLite Dialect = "sqlite"
)

// advisoryLockKey is the Postgres session lock for Apply. It is not a
// secret. Every process must use this same key. SQLite does not take it.
const advisoryLockKey int64 = 759826481032745

const (
	lockSQL   = "SELECT pg_advisory_lock($1)"
	unlockSQL = "SELECT pg_advisory_unlock($1)"
)

//go:embed migrations/postgres migrations/sqlite
var embedded embed.FS

// Apply runs pending migrations from the embedded SQL for dialect.
// Each file is one transaction. A file already recorded in
// schema_migrations is not run again, so Apply is idempotent. Applied
// files must not be edited; a change is a new version.
//
// Postgres holds a session advisory lock for the whole call and releases
// it before Apply returns, including on failure. The lock and the
// migrations use one connection. SQLite does not send that statement.
//
// Apply does not log SQL, destinations, or connection strings.
func Apply(ctx context.Context, db *sql.DB, dialect Dialect) error {
	return apply(ctx, db, dialect, embedded)
}

func apply(ctx context.Context, db *sql.DB, dialect Dialect, fsys fs.FS) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if db == nil {
		return errors.New("migrate: nil db")
	}
	if !knownDialect(dialect) {
		return errors.New("migrate: unknown dialect")
	}
	migrations, err := load(fsys, dialect)
	if err != nil {
		return err
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migrate: connection: %w", err)
	}
	defer func() {
		err = errors.Join(err, conn.Close())
	}()

	release, err := lock(ctx, conn, dialect)
	if err != nil {
		return err
	}
	applyErr := applyLocked(ctx, conn, dialect, migrations)
	err = errors.Join(applyErr, release())
	return err
}

func knownDialect(dialect Dialect) bool {
	return dialect == Postgres || dialect == SQLite
}

// lock takes the Postgres advisory lock on conn. The returned function
// releases it on that same session. SQLite returns a no-op release.
func lock(ctx context.Context, conn *sql.Conn, dialect Dialect) (func() error, error) {
	if dialect != Postgres {
		return func() error { return nil }, nil
	}
	// pg_advisory_lock returns void. Exec avoids scanning a void result.
	if _, err := conn.ExecContext(ctx, lockSQL, advisoryLockKey); err != nil {
		return nil, fmt.Errorf("migrate: advisory lock: %w", err)
	}
	return func() error {
		// A cancelled Apply must still unlock the session.
		_, err := conn.ExecContext(context.WithoutCancel(ctx), unlockSQL, advisoryLockKey)
		if err != nil {
			return fmt.Errorf("migrate: advisory unlock: %w", err)
		}
		return nil
	}, nil
}

func applyLocked(ctx context.Context, conn *sql.Conn, dialect Dialect, migrations []migration) error {
	if _, err := conn.ExecContext(ctx, schemaMigrationsSQL(dialect)); err != nil {
		return fmt.Errorf("migrate: schema_migrations: %w", err)
	}
	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return err
	}
	insertSQL := insertVersionSQL(dialect)
	for _, m := range migrations {
		if _, ok := applied[m.version]; ok {
			continue
		}
		if err := applyOne(ctx, conn, insertSQL, m); err != nil {
			return err
		}
	}
	return nil
}

func schemaMigrationsSQL(dialect Dialect) string {
	appliedType := "INTEGER"
	if dialect == Postgres {
		appliedType = "BIGINT"
	}
	return `CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at ` + appliedType + ` NOT NULL
)`
}

func insertVersionSQL(dialect Dialect) string {
	if dialect == Postgres {
		return "INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)"
	}
	return "INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)"
}

func appliedVersions(ctx context.Context, conn *sql.Conn) (applied map[string]struct{}, err error) {
	rows, err := conn.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("migrate: list versions: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("migrate: list versions: %w", cerr)
		}
	}()
	applied = make(map[string]struct{})
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("migrate: list versions: %w", err)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migrate: list versions: %w", err)
	}
	return applied, nil
}

func applyOne(ctx context.Context, conn *sql.Conn, insertSQL string, m migration) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migrate: %s: %w", m.version, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	for _, stmt := range m.statements {
		if err := rejectTxnControl(stmt); err != nil {
			return fmt.Errorf("migrate: %s: %w", m.version, err)
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: %s: %w", m.version, err)
		}
	}
	appliedAt := time.Now().UTC().UnixMilli()
	if _, err := tx.ExecContext(ctx, insertSQL, m.version, appliedAt); err != nil {
		return fmt.Errorf("migrate: %s: %w", m.version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate: %s: %w", m.version, err)
	}
	committed = true
	return nil
}

type migration struct {
	version    string
	statements []string
}

func load(fsys fs.FS, dialect Dialect) ([]migration, error) {
	root := path.Join("migrations", string(dialect))
	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		return nil, fmt.Errorf("migrate: read %s: %w", root, err)
	}
	migrations := make([]migration, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			return nil, fmt.Errorf("migrate: %s/%s is a directory", root, name)
		}
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		if !versionName(name) {
			return nil, fmt.Errorf("migrate: %s is not a versioned sql file", name)
		}
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("migrate: duplicate version %s", name)
		}
		seen[name] = struct{}{}
		body, err := fs.ReadFile(fsys, path.Join(root, name))
		if err != nil {
			return nil, fmt.Errorf("migrate: read %s: %w", name, err)
		}
		statements, err := splitSQL(string(body))
		if err != nil {
			return nil, fmt.Errorf("migrate: %s: %w", name, err)
		}
		if len(statements) == 0 {
			return nil, fmt.Errorf("migrate: %s has no statements", name)
		}
		migrations = append(migrations, migration{version: name, statements: statements})
	}
	slices.SortFunc(migrations, func(a, b migration) int {
		return strings.Compare(a.version, b.version)
	})
	return migrations, nil
}
