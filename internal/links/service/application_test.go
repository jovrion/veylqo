package service

import (
	"context"
	"database/sql"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jovrion/veylqo/internal/links/adapters"
	"github.com/jovrion/veylqo/internal/links/domain/link"
	"github.com/jovrion/veylqo/internal/migrate"

	_ "modernc.org/sqlite"
)

func TestSQLiteDSNWiresSQLiteAdapter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "veylqo.db")
	raw := "sqlite://file:" + filepath.ToSlash(path) + "?mode=rwc"
	opened := hookOpen(t, func(driver, dsn string) error {
		if driver != "sqlite" {
			t.Errorf("driver = %q, want sqlite", driver)
		}
		if strings.Contains(strings.ToLower(dsn), "postgres") || strings.Contains(dsn, "pgx") {
			t.Errorf("sqlite open received postgres dsn %q", dsn)
		}
		if !strings.Contains(dsn, "_txlock=immediate") || !strings.Contains(dsn, "_busy_timeout=5000") {
			t.Errorf("sqlite dsn missing pool params: %s", dsn)
		}
		return nil
	})

	wired, err := NewApplication(context.Background(), Config{DatabaseURL: raw})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := wired.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if len(opened.drivers) != 1 || opened.drivers[0] != "sqlite" {
		t.Fatalf("opened drivers = %v, want [sqlite]", opened.drivers)
	}
	if _, ok := wired.Store.(*adapters.SQLiteRepository); !ok {
		t.Fatalf("store = %T, want *adapters.SQLiteRepository", wired.Store)
	}
	if _, ok := wired.Store.(*adapters.PostgresRepository); ok {
		t.Fatal("sqlite DSN constructed the postgres adapter")
	}
	if _, ok := wired.Clicks.(*adapters.SQLiteClicks); !ok {
		t.Fatalf("clicks = %T, want *adapters.SQLiteClicks", wired.Clicks)
	}
	if _, ok := wired.Clicks.(*adapters.PostgresClicks); ok {
		t.Fatal("sqlite DSN wired postgres clicks")
	}
	if wired.DB == nil {
		t.Fatal("sqlite DSN did not open a pool")
	}
	if err := migrate.Apply(context.Background(), wired.DB, migrate.SQLite); err != nil {
		t.Fatal(err)
	}
	code, err := link.NewCode("Ab3xyZ9")
	if err != nil {
		t.Fatal(err)
	}
	n, err := wired.Clicks.Count(context.Background(), code)
	if err != nil || n != 0 {
		t.Fatalf("sqlite count = %d, %v", n, err)
	}
}

func TestPostgresDSNDoesNotOpenSQLite(t *testing.T) {
	const secret = "super-secret-password"
	raw := "postgres://veylqo:" + secret + "@192.0.2.10/veylqo?sslmode=disable"
	opened := hookOpen(t, func(driver, dsn string) error {
		if driver == "sqlite" || strings.HasPrefix(dsn, "file:") {
			t.Errorf("postgres DSN opened sqlite: driver %s dsn %s", driver, dsn)
		}
		return errors.New("open refused " + dsn)
	})

	_, err := NewApplication(context.Background(), Config{DatabaseURL: raw})
	if err == nil {
		t.Fatal("expected open error")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "192.0.2.10") || strings.Contains(err.Error(), raw) {
		t.Fatalf("error leaked database url: %v", err)
	}
	if len(opened.drivers) != 1 || opened.drivers[0] != "pgx" {
		t.Fatalf("opened drivers = %v, want [pgx]", opened.drivers)
	}
}

func TestNewApplicationRejectsBadURL(t *testing.T) {
	opened := hookOpen(t, func(string, string) error {
		t.Fatal("bad url opened a database")
		return nil
	})
	for _, raw := range []string{"", "sqlite://", "postgres://", "mysql://localhost/db", "https://example.com/a?token=secret"} {
		_, err := NewApplication(context.Background(), Config{DatabaseURL: raw})
		if err == nil {
			t.Fatalf("%q was accepted", raw)
		}
		if strings.Contains(err.Error(), "token=secret") || strings.Contains(err.Error(), "example.com") {
			t.Fatalf("error leaked url for %q: %v", raw, err)
		}
	}
	// A nil variable, not the nil literal: staticcheck SA1012 rewrites
	// NewApplication(nil) to context.TODO and would hide this rejection.
	var nilCtx context.Context
	if _, err := NewApplication(nilCtx, Config{DatabaseURL: "sqlite://file:x"}); err == nil {
		t.Fatal("nil context was accepted")
	}
	if len(opened.drivers) != 0 {
		t.Fatalf("opened drivers = %v", opened.drivers)
	}
}

func TestNewComponentTestApplicationUsesMemory(t *testing.T) {
	opened := hookOpen(t, func(driver, dsn string) error {
		t.Errorf("memory application opened %s %s", driver, dsn)
		return errors.New("opened")
	})
	wired, err := NewComponentTestApplication(Config{
		DatabaseURL: "postgres://veylqo:secret@192.0.2.10/veylqo",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := wired.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if len(opened.drivers) != 0 {
		t.Fatalf("opened drivers = %v", opened.drivers)
	}
	if wired.DB != nil {
		t.Fatal("memory application opened a pool")
	}
	if _, ok := wired.Store.(*adapters.MemoryRepository); !ok {
		t.Fatalf("store = %T, want *adapters.MemoryRepository", wired.Store)
	}
	if _, ok := wired.Store.(*adapters.SQLiteRepository); ok {
		t.Fatal("memory application wired sqlite")
	}
	if _, ok := wired.Store.(*adapters.PostgresRepository); ok {
		t.Fatal("memory application wired postgres")
	}
	if wired.Batcher == nil || wired.Commands.Shorten == nil || wired.Queries.Resolve == nil {
		t.Fatal("component application was not assembled")
	}
}

func TestServiceDoesNotImportDrivers(t *testing.T) {
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
			if path == "modernc.org/sqlite" || path == "github.com/jackc/pgx/v5/stdlib" || strings.HasPrefix(path, "github.com/jackc/pgx/") {
				t.Fatalf("%s imports %s", name, path)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no non-test service files")
	}
}

type openedDrivers struct {
	drivers []string
}

func hookOpen(t *testing.T, check func(driver, dsn string) error) *openedDrivers {
	t.Helper()
	opened := &openedDrivers{}
	prev := openSQL
	openSQL = func(driver, dsn string) (*sql.DB, error) {
		opened.drivers = append(opened.drivers, driver)
		if err := check(driver, dsn); err != nil {
			return nil, err
		}
		return prev(driver, dsn)
	}
	t.Cleanup(func() { openSQL = prev })
	return opened
}
