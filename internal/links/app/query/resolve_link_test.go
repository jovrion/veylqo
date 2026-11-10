package query

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"uuid"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/links/domain/link"
)

func TestResolveLink(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	const stored = "https://Example.COM/a?token=secret"

	active := mustLink(t, link.DatabaseLink{
		ID:          uuid.NewV7(),
		Code:        "Ab3xyZ9",
		Destination: stored,
		CreatedAt:   now.Add(-time.Hour),
		ExpiresAt:   now.Add(time.Hour),
	})
	expired := mustLink(t, link.DatabaseLink{
		ID:          uuid.NewV7(),
		Code:        "Ab3xyZ9",
		Destination: stored,
		CreatedAt:   now.Add(-2 * time.Hour),
		ExpiresAt:   now,
	})
	deleted := mustLink(t, link.DatabaseLink{
		ID:          uuid.NewV7(),
		Code:        "kept-slug",
		Destination: stored,
		Custom:      true,
		CreatedAt:   now.Add(-time.Hour),
		DeletedAt:   now.Add(-time.Minute),
	})

	tests := []struct {
		name     string
		code     string
		row      link.Link
		getErr   error
		at       time.Time
		want     string
		wantSlug apperrors.Slug
		noGet    bool
	}{
		{
			name: "active returns stored destination",
			code: "Ab3xyZ9",
			row:  active,
			at:   now,
			want: stored,
		},
		{
			name:     "missing is not-found",
			code:     "Ab3xyZ9",
			getErr:   &link.NotFoundError{},
			at:       now,
			wantSlug: apperrors.NotFound,
		},
		{
			name:     "expired is gone",
			code:     "Ab3xyZ9",
			row:      expired,
			at:       now,
			wantSlug: apperrors.Gone,
		},
		{
			name:     "deleted is gone",
			code:     "kept-slug",
			row:      deleted,
			at:       now,
			wantSlug: apperrors.Gone,
		},
		{
			name:     "uuid is not-found and is not looked up",
			code:     "550e8400-e29b-41d4-a716-446655440000",
			at:       now,
			wantSlug: apperrors.NotFound,
			noGet:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &resolveRepo{row: tt.row, err: tt.getErr}
			handler := NewResolveLinkHandler(repo, func() time.Time { return tt.at })
			if handler.Name() != "ResolveLink" {
				t.Fatalf("Name() = %q", handler.Name())
			}

			got, err := handler.Handle(context.Background(), ResolveLink{Code: tt.code})
			if tt.wantSlug == "" {
				if err != nil {
					t.Fatalf("Handle() error = %v", err)
				}
				if got.Destination.String() != tt.want {
					t.Fatalf("destination = %q, want stored destination unchanged", got.Destination.String())
				}
			} else {
				assertSlug(t, err, tt.wantSlug)
				if got.Destination.String() != "" {
					t.Fatal("failure returned a destination")
				}
				if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "Example.COM") || strings.Contains(err.Error(), "?") {
					t.Fatalf("error leaked the destination: %s", err.Error())
				}
			}
			if tt.noGet && repo.gets != 0 {
				t.Fatalf("Get calls = %d, want 0", repo.gets)
			}
			if !tt.noGet && repo.gets != 1 {
				t.Fatalf("Get calls = %d, want 1", repo.gets)
			}
			if repo.writes != 0 {
				t.Fatalf("writes = %d, want 0", repo.writes)
			}
		})
	}
}

func TestResolveLinkMissingSentinelIsNotFound(t *testing.T) {
	t.Parallel()

	repo := &resolveRepo{err: link.ErrNotFound}
	handler := NewResolveLinkHandler(repo, func() time.Time {
		return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	})
	_, err := handler.Handle(context.Background(), ResolveLink{Code: "Ab3xyZ9"})
	assertSlug(t, err, apperrors.NotFound)
	if !errors.Is(err, link.ErrNotFound) {
		t.Fatalf("Handle() = %v, want wrapped not-found", err)
	}
}

func TestResolveLinkStoreErrorIsNotASlug(t *testing.T) {
	t.Parallel()

	storeErr := errors.New("store unavailable")
	repo := &resolveRepo{err: storeErr}
	handler := NewResolveLinkHandler(repo, func() time.Time {
		return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	})
	_, err := handler.Handle(context.Background(), ResolveLink{Code: "Ab3xyZ9"})
	if !errors.Is(err, storeErr) {
		t.Fatalf("Handle() = %v, want store error", err)
	}
	if slug, ok := apperrors.As(err); ok {
		t.Fatalf("store error became slug %q", slug.Slug())
	}
	if repo.writes != 0 {
		t.Fatalf("writes = %d, want 0", repo.writes)
	}
}

func TestResolveLinkRejectsBrokenClockAndNilDeps(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	row := mustLink(t, link.DatabaseLink{
		ID:          uuid.NewV7(),
		Code:        "Ab3xyZ9",
		Destination: "https://example.com/a",
		CreatedAt:   now,
	})
	repo := &resolveRepo{row: row}

	_, err := NewResolveLinkHandler(repo, func() time.Time { return time.Time{} }).Handle(context.Background(), ResolveLink{Code: "Ab3xyZ9"})
	assertSlug(t, err, apperrors.IncorrectInput)

	_, err = NewResolveLinkHandler(nil, func() time.Time { return now }).Handle(context.Background(), ResolveLink{Code: "Ab3xyZ9"})
	if err == nil || strings.Contains(err.Error(), "example.com") {
		t.Fatalf("nil repository = %v", err)
	}
	_, err = NewResolveLinkHandler(repo, nil).Handle(context.Background(), ResolveLink{Code: "Ab3xyZ9"})
	if err == nil {
		t.Fatal("nil clock returned a destination")
	}
}

func TestResolveLinkDoesNotImportHTTP(t *testing.T) {
	t.Parallel()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files int
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files++
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if strings.Contains(text, "ClickRepository") || strings.Contains(text, ".Add(") {
			t.Fatalf("%s records a click or writes a link", name)
		}
		file, err := parser.ParseFile(fset, name, body, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if path == "net/http" || strings.HasPrefix(path, "net/http/") ||
				path == "database/sql" || strings.HasPrefix(path, "database/sql/") {
				t.Fatalf("%s imports %s", name, path)
			}
		}
	}
	if files == 0 {
		t.Fatal("no query files found")
	}
}

func assertSlug(t *testing.T, err error, want apperrors.Slug) {
	t.Helper()
	slug, ok := apperrors.As(err)
	if !ok {
		t.Fatalf("Handle() = %v, want slug %s", err, want)
	}
	if slug.Slug() != want {
		t.Fatalf("slug = %s, want %s", slug.Slug(), want)
	}
	if !errors.Is(err, slugSentinel(want)) {
		t.Fatalf("errors.Is slug %s = false", want)
	}
}

func slugSentinel(slug apperrors.Slug) error {
	switch slug {
	case apperrors.NotFound:
		return apperrors.NewNotFound("", nil)
	case apperrors.Gone:
		return apperrors.NewGone("", nil)
	case apperrors.IncorrectInput:
		return apperrors.NewIncorrectInput("", nil)
	default:
		return nil
	}
}

func mustLink(t *testing.T, row link.DatabaseLink) link.Link {
	t.Helper()
	got, err := link.UnmarshalLinkFromDatabase(row)
	if err != nil {
		t.Fatalf("UnmarshalLinkFromDatabase() error = %v", err)
	}
	return got
}

// resolveRepo is a stand-in for the domain repository. Writes fail the
// test if the query records a click or mutates a link.
type resolveRepo struct {
	row    link.Link
	err    error
	gets   int
	writes int
}

func (r *resolveRepo) Get(_ context.Context, code link.Code) (link.Link, error) {
	r.gets++
	if r.err != nil {
		return link.Link{}, r.err
	}
	if r.row.Code().String() != code.String() {
		return link.Link{}, &link.NotFoundError{}
	}
	return r.row, nil
}

func (r *resolveRepo) Add(context.Context, link.Link) error {
	r.writes++
	return errors.New("resolve must not add")
}

func (r *resolveRepo) GetByDestination(context.Context, link.Destination) (link.Link, error) {
	r.writes++
	return link.Link{}, errors.New("resolve must not search by destination")
}

func (r *resolveRepo) Update(context.Context, link.Code, func(*link.Link) error) error {
	r.writes++
	return errors.New("resolve must not update")
}

var _ link.Repository = (*resolveRepo)(nil)
