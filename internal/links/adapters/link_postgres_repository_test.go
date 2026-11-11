package adapters

import (
	"context"
	"database/sql"
	"errors"
	"go/parser"
	"go/token"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jovrion/veylqo/internal/links/domain/link"
	"github.com/jovrion/veylqo/internal/migrate"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresRepository(t *testing.T) {
	db := openPostgres(t)
	RunLinkRepositorySuite(t, func(t *testing.T) link.Repository {
		t.Helper()
		if _, err := db.ExecContext(context.Background(), `TRUNCATE links`); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		return NewPostgresRepository(db)
	})
}

func TestPostgresStoresIDAsUUID(t *testing.T) {
	db := openPostgres(t)
	repo := NewPostgresRepository(db)
	created := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	expires := created.Add(time.Hour)
	item := mustSuiteLink(t, "Ab3xyZ9", "https://example.com/a", false, created, expires)
	if err := repo.Add(context.Background(), item); err != nil {
		t.Fatal(err)
	}

	var idType string
	err := db.QueryRowContext(context.Background(), `SELECT pg_typeof(id)::text FROM links WHERE code = $1`, item.Code().String()).
		Scan(&idType)
	if err != nil {
		t.Fatal(err)
	}
	if idType != "uuid" {
		t.Fatalf("id type = %s, want uuid", idType)
	}

	var id uuid.UUID
	err = db.QueryRowContext(context.Background(), `SELECT id FROM links WHERE code = $1`, item.Code().String()).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	if id != item.ID() {
		t.Fatal("scanned id is not the standard uuid value")
	}
}

func TestPostgresAdapterDoesNotImportDriver(t *testing.T) {
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
			if path == "github.com/jackc/pgx" || strings.HasPrefix(path, "github.com/jackc/pgx/") {
				t.Fatalf("%s imports %s", name, path)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no non-test adapter files")
	}
}

func TestPostgresDSNWithSearchPath(t *testing.T) {
	got := postgresDSNWithSearchPath("postgres://veylqo@/veylqo?sslmode=disable", "veylqo_abc")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("options") != "-csearch_path=veylqo_abc" || u.Query().Get("sslmode") != "disable" {
		t.Fatalf("url dsn = %s", got)
	}
	got = postgresDSNWithSearchPath("host=localhost user=veylqo", "veylqo_abc")
	if got != "host=localhost user=veylqo options=-csearch_path=veylqo_abc" {
		t.Fatalf("keyword dsn = %s", got)
	}
}

func TestPostgresConstraintReason(t *testing.T) {
	codeErr := errors.New(`ERROR: duplicate key value violates unique constraint "links_code" (SQLSTATE 23505)`)
	reason, ok := postgresConstraintReason(codeErr)
	if !ok || reason != "code" {
		t.Fatalf("code constraint = %q %v", reason, ok)
	}
	destErr := errors.New(`ERROR: duplicate key value violates unique constraint "links_destination_hash_plain" (SQLSTATE 23505)`)
	reason, ok = postgresConstraintReason(destErr)
	if !ok || reason != "destination" {
		t.Fatalf("destination constraint = %q %v", reason, ok)
	}
	if _, ok := postgresConstraintReason(errors.New("SQLSTATE 23505")); ok {
		t.Fatal("unique violation without a known index was mapped")
	}
	if _, ok := postgresConstraintReason(errors.New(`constraint "links_code"`)); ok {
		t.Fatal("index name without SQLSTATE 23505 was mapped")
	}
}

func openPostgres(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("VEYLQO_TEST_PG_DSN"))
	if dsn == "" {
		t.Skip("VEYLQO_TEST_PG_DSN is unset")
	}
	schema := "veylqo_" + strings.ReplaceAll(uuid.NewV7().String(), "-", "")
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := admin.PingContext(ctx); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
		if err := admin.Close(); err != nil {
			t.Error(err)
		}
	})

	db, err := sql.Open("pgx", postgresDSNWithSearchPath(dsn, schema))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(ctx, db, migrate.Postgres); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// postgresDSNWithSearchPath returns dsn with search_path set to schema.
// schema is a generated identifier, not operator input. A URL DSN and a
// keyword DSN are both accepted. The adapter does not parse DSNs.
func postgresDSNWithSearchPath(dsn, schema string) string {
	opt := "-csearch_path=" + schema
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return dsn
		}
		q := u.Query()
		if existing := q.Get("options"); existing != "" {
			opt = existing + " " + opt
		}
		q.Set("options", opt)
		u.RawQuery = q.Encode()
		return u.String()
	}
	return dsn + " options=" + opt
}
