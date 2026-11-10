package adapters

import (
	"context"
	"database/sql"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jovrion/veylqo/internal/links/domain/link"
	"github.com/jovrion/veylqo/internal/migrate"

	_ "modernc.org/sqlite"
)

func TestSQLiteRepository(t *testing.T) {
	RunLinkRepositorySuite(t, func(t *testing.T) link.Repository {
		t.Helper()
		db := openSQLite(t)
		if err := migrate.Apply(context.Background(), db, migrate.SQLite); err != nil {
			t.Fatalf("migrate: %v", err)
		}
		return NewSQLiteRepository(db)
	})
}

func TestSQLiteTimestampsAreUnixMillis(t *testing.T) {
	db := openSQLite(t)
	if err := migrate.Apply(context.Background(), db, migrate.SQLite); err != nil {
		t.Fatal(err)
	}
	repo := NewSQLiteRepository(db)
	created := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	expires := created.Add(time.Hour)
	item := mustSuiteLink(t, "Ab3xyZ9", "https://example.com/a", false, created, expires)
	if err := repo.Add(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	deletedAt := created.Add(time.Minute)
	if err := repo.Update(context.Background(), item.Code(), func(row *link.Link) error {
		return row.Delete(deletedAt)
	}); err != nil {
		t.Fatal(err)
	}

	var (
		createdMS int64
		expiresMS sql.NullInt64
		deletedMS sql.NullInt64
	)
	err := db.QueryRow(`SELECT created_at, expires_at, deleted_at FROM links WHERE code = ?`, item.Code().String()).
		Scan(&createdMS, &expiresMS, &deletedMS)
	if err != nil {
		t.Fatal(err)
	}
	if createdMS != created.UnixMilli() || !expiresMS.Valid || expiresMS.Int64 != expires.UnixMilli() {
		t.Fatalf("timestamps = %d %v, want unix milliseconds", createdMS, expiresMS)
	}
	if !deletedMS.Valid || deletedMS.Int64 != deletedAt.UnixMilli() {
		t.Fatalf("deleted_at = %v, want %d", deletedMS, deletedAt.UnixMilli())
	}
}

func TestSQLiteAdapterDoesNotImportDriver(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	seen := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		seen++
		file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("import %s: %v", name, err)
			}
			if path == "modernc.org/sqlite" || strings.HasPrefix(path, "modernc.org/sqlite/") {
				t.Fatalf("%s imports %s", name, path)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no non-test adapter files")
	}
}

func openSQLite(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "veylqo.db")
	dsn := "file:" + filepath.ToSlash(path) + "?mode=rwc&_busy_timeout=5000&_txlock=immediate"
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
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	return db
}
