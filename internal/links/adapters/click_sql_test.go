package adapters

import (
	"context"
	"strings"
	"testing"

	"github.com/jovrion/veylqo/internal/links/domain/link"
	"github.com/jovrion/veylqo/internal/migrate"
)

func TestSQLiteClicks(t *testing.T) {
	db := openSQLite(t)
	if err := migrate.Apply(context.Background(), db, migrate.SQLite); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clicks := NewSQLiteClicks(db)
	code := mustClickCode(t, "Ab3xyZ9")
	ctx := context.Background()

	n, err := clicks.Count(ctx, code)
	if err != nil || n != 0 {
		t.Fatalf("missing count = %d, %v", n, err)
	}
	if err := clicks.Add(ctx, code, 2); err != nil {
		t.Fatal(err)
	}
	if err := clicks.Add(ctx, code, 3); err != nil {
		t.Fatal(err)
	}
	n, err = clicks.Count(ctx, code)
	if err != nil || n != 5 {
		t.Fatalf("count = %d, %v", n, err)
	}
	if err := clicks.Add(ctx, code, 0); err == nil {
		t.Fatal("non-positive n was stored")
	}
	n, err = clicks.Count(ctx, code)
	if err != nil || n != 5 {
		t.Fatalf("count after reject = %d, %v", n, err)
	}
}

func TestClickSQLDialectsDiffer(t *testing.T) {
	sqlite := NewSQLiteClicks(nil)
	postgres := NewPostgresClicks(nil)
	if strings.Contains(sqlite.add, "$1") || !strings.Contains(sqlite.add, "?") {
		t.Fatal("sqlite clicks must use ?")
	}
	if strings.Contains(postgres.add, "?") || !strings.Contains(postgres.add, "$1") {
		t.Fatal("postgres clicks must use $1")
	}
	if sqlite.count == postgres.count {
		t.Fatal("click dialects share a count query")
	}
}

func mustClickCode(t *testing.T, raw string) link.Code {
	t.Helper()
	code, err := link.NewCode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return code
}
