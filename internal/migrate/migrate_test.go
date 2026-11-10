package migrate

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	_ "modernc.org/sqlite"
)

func TestApplySQLiteEmbedded(t *testing.T) {
	db := openTempSQLite(t)
	ctx := context.Background()
	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	assertSQLiteSchema(t, db)
	first := appliedAt(t, db)

	if err := Apply(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	second := appliedAt(t, db)
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("versions first %d second %d", len(first), len(second))
	}
	for version, at := range first {
		got, ok := second[version]
		if !ok {
			t.Fatalf("second apply dropped %s", version)
		}
		if got != at {
			t.Fatalf("second apply rewrote %s applied_at %d -> %d", version, at, got)
		}
	}
	var n int
	if err := db.QueryRow("SELECT count(*) FROM links").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("links = %d, want 1 after second apply", n)
	}
}

func TestApplySQLiteRollsBackFailedMigration(t *testing.T) {
	db := openTempSQLite(t)
	files := fstest.MapFS{
		"migrations/sqlite/0001_ok.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE kept (id INTEGER PRIMARY KEY);"),
		},
		"migrations/sqlite/0002_bad.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE rolled (id INTEGER PRIMARY KEY);\nINSERT INTO missing (id) VALUES (1);"),
		},
	}
	err := apply(context.Background(), db, SQLite, files)
	if err == nil {
		t.Fatal("bad migration returned nil error")
	}
	if strings.Contains(err.Error(), "INSERT INTO") {
		t.Fatalf("error includes sql: %v", err)
	}
	if !tableExists(t, db, "kept") {
		t.Fatal("committed migration was rolled back")
	}
	if tableExists(t, db, "rolled") {
		t.Fatal("failed migration was committed")
	}
	versions := appliedAt(t, db)
	if _, ok := versions["0001_ok.sql"]; !ok {
		t.Fatal("successful version was not recorded")
	}
	if _, ok := versions["0002_bad.sql"]; ok {
		t.Fatal("failed version was recorded")
	}

	fixed := fstest.MapFS{
		"migrations/sqlite/0001_ok.sql": files["migrations/sqlite/0001_ok.sql"],
		"migrations/sqlite/0002_bad.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE rolled (id INTEGER PRIMARY KEY);"),
		},
	}
	if err := apply(context.Background(), db, SQLite, fixed); err != nil {
		t.Fatal(err)
	}
	if !tableExists(t, db, "rolled") {
		t.Fatal("retry did not apply the failed version")
	}
	if _, ok := appliedAt(t, db)["0001_ok.sql"]; !ok {
		t.Fatal("retry re-ran or dropped the first version")
	}
}

func TestApplySQLiteDoesNotSendAdvisoryLock(t *testing.T) {
	db := openTempSQLite(t)
	if err := Apply(context.Background(), db, SQLite); err != nil {
		t.Fatal(err)
	}
	var missing string
	err := db.QueryRow("SELECT pg_advisory_lock(1)").Scan(&missing)
	if err == nil {
		t.Fatal("sqlite executed pg_advisory_lock")
	}
}

func TestApplyRejectsUnknownDialect(t *testing.T) {
	db := openTempSQLite(t)
	err := Apply(context.Background(), db, Dialect("mysql"))
	if err == nil || !strings.Contains(err.Error(), "unknown dialect") {
		t.Fatalf("err = %v", err)
	}
}

func TestApplyCancelledContext(t *testing.T) {
	db := openTempSQLite(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Apply(ctx, db, SQLite)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestEmbeddedDialectsShareVersions(t *testing.T) {
	pg, err := load(embedded, Postgres)
	if err != nil {
		t.Fatal(err)
	}
	lite, err := load(embedded, SQLite)
	if err != nil {
		t.Fatal(err)
	}
	if len(pg) == 0 || len(pg) != len(lite) {
		t.Fatalf("postgres %d sqlite %d", len(pg), len(lite))
	}
	var pgSQL, liteSQL strings.Builder
	for i := range pg {
		if pg[i].version != lite[i].version {
			t.Fatalf("version %d postgres %s sqlite %s", i, pg[i].version, lite[i].version)
		}
		for _, stmt := range pg[i].statements {
			pgSQL.WriteString(stmt)
			pgSQL.WriteByte('\n')
		}
		for _, stmt := range lite[i].statements {
			liteSQL.WriteString(stmt)
			liteSQL.WriteByte('\n')
		}
	}
	if !strings.Contains(pgSQL.String(), "UUID") {
		t.Fatal("postgres migrations do not use uuid")
	}
	if strings.Contains(liteSQL.String(), "UUID") {
		t.Fatal("sqlite migrations use uuid")
	}
	if strings.Contains(pgSQL.String(), "pg_advisory_lock") || strings.Contains(liteSQL.String(), "pg_advisory_lock") {
		t.Fatal("advisory lock belongs in Apply, not in a migration file")
	}
}

func TestGoModHasNoMigrationTool(t *testing.T) {
	root := moduleRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, banned := range []string{
		"github.com/pressly/goose",
		"github.com/golang-migrate/migrate",
		"github.com/amacneil/dbmate",
	} {
		if strings.Contains(text, banned) {
			t.Fatalf("go.mod requires %s", banned)
		}
	}
}

func openTempSQLite(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "veylqo.db")
	dsn := "file:" + filepath.ToSlash(path) + "?mode=rwc&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	db.SetMaxOpenConns(1)
	return db
}

func assertSQLiteSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, table := range []string{"schema_migrations", "links", "clicks"} {
		if !tableExists(t, db, table) {
			t.Fatalf("missing table %s", table)
		}
	}
	if _, err := db.Exec(`INSERT INTO links (
    id, code, destination, destination_hash, custom, created_at
) VALUES (?, ?, ?, ?, ?, ?)`,
		"018f1b3a-7c2a-7000-8000-000000000001",
		"Ab3xyZ9",
		"https://example.com/a",
		"hash",
		0,
		time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC).UnixMilli(),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO links (
    id, code, destination, destination_hash, custom, created_at
) VALUES (?, ?, ?, ?, ?, ?)`,
		"018f1b3a-7c2a-7000-8000-000000000002",
		"Bb3xyZ9",
		"https://example.com/b",
		"hash",
		0,
		1,
	); err == nil {
		t.Fatal("partial unique index allowed a second plain destination hash")
	}
	if _, err := db.Exec(`INSERT INTO clicks (code, click_count) VALUES (?, ?)`, "Ab3xyZ9", 1); err != nil {
		t.Fatal(err)
	}
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func appliedAt(t *testing.T, db *sql.DB) map[string]int64 {
	t.Helper()
	rows, err := db.Query("SELECT version, applied_at FROM schema_migrations")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := make(map[string]int64)
	for rows.Next() {
		var version string
		var at int64
		if err := rows.Scan(&version, &at); err != nil {
			t.Fatal(err)
		}
		out[version] = at
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
