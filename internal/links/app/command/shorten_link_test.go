package command

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

func TestShorten(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	t.Run("same URL without a slug returns the same code", func(t *testing.T) {
		t.Parallel()
		repo := newMemRepo()
		codes := &seqCodes{codes: []string{"Ab3xyZ9", "zzzzzzz"}}
		h := handler(t, repo, codes, clock, "example.com")

		first, err := h.Handle(context.Background(), ShortenLink{
			URL: "https://Example.COM/a?utm_source=one",
		})
		if err != nil {
			t.Fatalf("first Handle() error = %v", err)
		}
		second, err := h.Handle(context.Background(), ShortenLink{
			URL: "https://example.com/a?utm_source=two&fbclid=abc",
		})
		if err != nil {
			t.Fatalf("second Handle() error = %v", err)
		}
		if first.Code.String() != "Ab3xyZ9" || second.Code.String() != first.Code.String() {
			t.Fatalf("codes = %s, %s; want one shared code", first.Code.String(), second.Code.String())
		}
		if codes.n != 1 {
			t.Fatalf("generator calls = %d, want 1", codes.n)
		}
		if repo.adds != 1 {
			t.Fatalf("adds = %d, want 1", repo.adds)
		}
		stored := repo.byCode["Ab3xyZ9"]
		if strings.Contains(stored.Destination().String(), "utm_") || strings.Contains(stored.Destination().String(), "fbclid") {
			t.Fatal("stored destination kept a tracking query")
		}
		if stored.ID() == uuid.Nil() || stored.ID().String() == first.Code.String() {
			t.Fatal("result exposed the internal id or stored a nil id")
		}
		if strings.Contains(first.Code.String(), "-") {
			t.Fatalf("public code %s looks like a UUID", first.Code.String())
		}
	})

	t.Run("query order does not mint a second code", func(t *testing.T) {
		t.Parallel()
		repo := newMemRepo()
		codes := &seqCodes{codes: []string{"Ab3xyZ9", "zzzzzzz"}}
		h := handler(t, repo, codes, clock, "example.com")

		first, err := h.Handle(context.Background(), ShortenLink{URL: "https://example.com/a?b=1&a=2"})
		if err != nil {
			t.Fatal(err)
		}
		second, err := h.Handle(context.Background(), ShortenLink{URL: "https://example.com/a?a=2&b=1"})
		if err != nil {
			t.Fatal(err)
		}
		if first.Code.String() != second.Code.String() {
			t.Fatalf("codes = %s, %s", first.Code.String(), second.Code.String())
		}
	})

	t.Run("host outside the allowlist is incorrect-input", func(t *testing.T) {
		t.Parallel()
		repo := newMemRepo()
		codes := &seqCodes{codes: []string{"Ab3xyZ9"}}
		h := handler(t, repo, codes, clock, "example.com")

		_, err := h.Handle(context.Background(), ShortenLink{
			URL: "https://evil.test/path?token=secret",
		})
		assertSlug(t, err, apperrors.IncorrectInput)
		if repo.adds != 0 || codes.n != 0 {
			t.Fatalf("rejected host still stored or generated: adds=%d codes=%d", repo.adds, codes.n)
		}
		text := err.Error()
		if strings.Contains(text, "evil.test") || strings.Contains(text, "token") || strings.Contains(text, "://") {
			t.Fatalf("error leaked the destination: %s", text)
		}
	})

	t.Run("empty allowlist fails closed", func(t *testing.T) {
		t.Parallel()
		repo := newMemRepo()
		allow, err := NewAllowlist(nil)
		if err != nil {
			t.Fatal(err)
		}
		h := NewShortenLinkHandler(repo, link.DefaultCleaner(), allow, &seqCodes{codes: []string{"Ab3xyZ9"}}, clock)
		_, err = h.Handle(context.Background(), ShortenLink{URL: "https://example.com/a"})
		assertSlug(t, err, apperrors.IncorrectInput)
		if repo.adds != 0 {
			t.Fatal("empty allowlist stored a link")
		}
	})

	t.Run("private host on the allowlist is still incorrect-input", func(t *testing.T) {
		t.Parallel()
		repo := newMemRepo()
		h := handler(t, repo, &seqCodes{codes: []string{"Ab3xyZ9"}}, clock, "127.0.0.1")
		_, err := h.Handle(context.Background(), ShortenLink{URL: "http://127.0.0.1/admin?token=secret"})
		assertSlug(t, err, apperrors.IncorrectInput)
		if !errors.Is(err, link.ErrInvalidDestination) {
			t.Fatalf("Handle() = %v, want destination validation", err)
		}
		if strings.Contains(err.Error(), "127.0.0.1") || strings.Contains(err.Error(), "token") {
			t.Fatalf("error leaked the destination: %s", err.Error())
		}
		if repo.adds != 0 {
			t.Fatal("private host was stored")
		}
	})

	t.Run("clean false does not collapse tracking parameters", func(t *testing.T) {
		t.Parallel()
		repo := newMemRepo()
		codes := &seqCodes{codes: []string{"Ab3xyZ9", "Bc4yzA8"}}
		h := handler(t, repo, codes, clock, "example.com")
		first, err := h.Handle(context.Background(), ShortenLink{
			URL:       "https://example.com/a?utm_source=one",
			SkipClean: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		second, err := h.Handle(context.Background(), ShortenLink{
			URL:       "https://example.com/a?utm_source=two",
			SkipClean: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if first.Code.String() == second.Code.String() {
			t.Fatal("clean: false reused a code for different tracking queries")
		}
		if !strings.Contains(repo.byCode[first.Code.String()].Destination().String(), "utm_source=one") {
			t.Fatal("clean: false stripped a tracking parameter")
		}
	})

	t.Run("custom slug does not reuse", func(t *testing.T) {
		t.Parallel()
		repo := newMemRepo()
		codes := &seqCodes{codes: []string{"Ab3xyZ9"}}
		h := handler(t, repo, codes, clock, "example.com")
		first, err := h.Handle(context.Background(), ShortenLink{URL: "https://example.com/a"})
		if err != nil {
			t.Fatal(err)
		}
		second, err := h.Handle(context.Background(), ShortenLink{URL: "https://example.com/a", Slug: "my-slug"})
		if err != nil {
			t.Fatal(err)
		}
		if second.Code.String() != "my-slug" || second.Code.String() == first.Code.String() {
			t.Fatalf("custom code = %s, generated = %s", second.Code.String(), first.Code.String())
		}
		if codes.n != 1 {
			t.Fatalf("generator calls = %d, want 1", codes.n)
		}
	})

	t.Run("expiry on the request does not reuse", func(t *testing.T) {
		t.Parallel()
		repo := newMemRepo()
		codes := &seqCodes{codes: []string{"Ab3xyZ9", "Bc4yzA8"}}
		h := handler(t, repo, codes, clock, "example.com")
		first, err := h.Handle(context.Background(), ShortenLink{URL: "https://example.com/a"})
		if err != nil {
			t.Fatal(err)
		}
		_, err = h.Handle(context.Background(), ShortenLink{
			URL:       "https://example.com/a",
			ExpiresAt: now.Add(time.Hour),
		})
		assertSlug(t, err, apperrors.Conflict)
		if _, ok := repo.byCode[first.Code.String()]; !ok {
			t.Fatal("original link disappeared")
		}
	})

	t.Run("existing expiry is not reused", func(t *testing.T) {
		t.Parallel()
		repo := newMemRepo()
		existing := mustLink(t, link.DatabaseLink{
			ID:          uuid.NewV7(),
			Code:        "Ab3xyZ9",
			Destination: "https://example.com/a",
			CreatedAt:   now.Add(-time.Hour),
			ExpiresAt:   now.Add(time.Hour),
		})
		if err := repo.Add(context.Background(), existing); err != nil {
			t.Fatal(err)
		}
		repo.adds = 0
		h := handler(t, repo, &seqCodes{codes: []string{"Bc4yzA8"}}, clock, "example.com")
		_, err := h.Handle(context.Background(), ShortenLink{URL: "https://example.com/a"})
		assertSlug(t, err, apperrors.Conflict)
	})

	t.Run("code collision retries", func(t *testing.T) {
		t.Parallel()
		repo := newMemRepo()
		taken := mustLink(t, link.DatabaseLink{
			ID:          uuid.NewV7(),
			Code:        "Ab3xyZ9",
			Destination: "https://example.com/other",
			CreatedAt:   now.Add(-time.Hour),
		})
		if err := repo.Add(context.Background(), taken); err != nil {
			t.Fatal(err)
		}
		repo.adds = 0
		codes := &seqCodes{codes: []string{"Ab3xyZ9", "Bc4yzA8"}}
		h := handler(t, repo, codes, clock, "example.com")
		got, err := h.Handle(context.Background(), ShortenLink{URL: "https://example.com/a"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Code.String() != "Bc4yzA8" {
			t.Fatalf("code = %s, want the retried code", got.Code.String())
		}
		if codes.n != 2 {
			t.Fatalf("generator calls = %d, want 2", codes.n)
		}
	})

	t.Run("destination conflict after a race reuses the winner", func(t *testing.T) {
		t.Parallel()
		repo := newMemRepo()
		winner := mustLink(t, link.DatabaseLink{
			ID:          uuid.NewV7(),
			Code:        "Winner1",
			Destination: "https://example.com/a",
			CreatedAt:   now.Add(-time.Minute),
		})
		repo.Add(context.Background(), winner)
		repo.adds = 0
		repo.hideDestination = true
		h := handler(t, repo, &seqCodes{codes: []string{"Bc4yzA8"}}, clock, "example.com")
		got, err := h.Handle(context.Background(), ShortenLink{URL: "https://example.com/a"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Code.String() != "Winner1" {
			t.Fatalf("code = %s, want the raced winner", got.Code.String())
		}
	})

	t.Run("reserved slug is incorrect-input", func(t *testing.T) {
		t.Parallel()
		repo := newMemRepo()
		h := handler(t, repo, &seqCodes{}, clock, "example.com")
		_, err := h.Handle(context.Background(), ShortenLink{URL: "https://example.com/a", Slug: "API"})
		assertSlug(t, err, apperrors.IncorrectInput)
		if repo.adds != 0 {
			t.Fatal("reserved slug was stored")
		}
	})

	t.Run("javascript scheme is incorrect-input", func(t *testing.T) {
		t.Parallel()
		repo := newMemRepo()
		h := handler(t, repo, &seqCodes{codes: []string{"Ab3xyZ9"}}, clock, "example.com")
		_, err := h.Handle(context.Background(), ShortenLink{URL: "javascript:alert(1)"})
		assertSlug(t, err, apperrors.IncorrectInput)
		if repo.adds != 0 {
			t.Fatal("non-http destination was stored")
		}
	})
}

func TestShortenAllowlistRejectsBadHost(t *testing.T) {
	t.Parallel()
	for _, host := range []string{"", "https://example.com", "example.com/a", "exämple.com", "*.example.com", "example.com:443"} {
		if _, err := NewAllowlist([]string{host}); err == nil {
			t.Fatalf("NewAllowlist(%q) succeeded", host)
		}
	}
	allow, err := NewAllowlist([]string{"Example.COM.", "2001:db8::1"})
	if err != nil {
		t.Fatal(err)
	}
	if !allow.Allows("example.com") || !allow.Allows("2001:db8::1") {
		t.Fatal("canonical hosts were rejected")
	}
	if allow.Allows("www.example.com") {
		t.Fatal("subdomain matched an exact host")
	}
}

func TestShortenNilDeps(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	allow, err := NewAllowlist([]string{"example.com"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewShortenLinkHandler(nil, link.DefaultCleaner(), allow, &seqCodes{}, func() time.Time { return now }).Handle(context.Background(), ShortenLink{URL: "https://example.com/a"})
	if err == nil {
		t.Fatal("nil repository succeeded")
	}
	_, err = NewShortenLinkHandler(newMemRepo(), link.DefaultCleaner(), allow, &seqCodes{}, nil).Handle(context.Background(), ShortenLink{URL: "https://example.com/a"})
	if err == nil {
		t.Fatal("nil clock succeeded")
	}
}

func TestShortenDoesNotImportHTTPOrSQL(t *testing.T) {
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
		if strings.Contains(text, "slog.") || strings.Contains(text, "log.") {
			t.Fatalf("%s logs from the command", name)
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
			switch {
			case path == "net/http" || strings.HasPrefix(path, "net/http/"):
				t.Fatalf("%s imports %s", name, path)
			case path == "database/sql" || strings.HasPrefix(path, "database/sql/"):
				t.Fatalf("%s imports %s", name, path)
			case path == "log" || path == "log/slog":
				t.Fatalf("%s imports %s", name, path)
			}
		}
	}
	if files == 0 {
		t.Fatal("no command files found")
	}
}

func handler(t *testing.T, repo link.Repository, codes CodeGenerator, now func() time.Time, hosts ...string) ShortenLinkHandler {
	t.Helper()
	allow, err := NewAllowlist(hosts)
	if err != nil {
		t.Fatal(err)
	}
	return NewShortenLinkHandler(repo, link.DefaultCleaner(), allow, codes, now)
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
	case apperrors.IncorrectInput:
		return apperrors.NewIncorrectInput("", nil)
	case apperrors.Conflict:
		return apperrors.NewConflict("", nil)
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

type seqCodes struct {
	codes []string
	n     int
}

func (s *seqCodes) NewCode(context.Context) (string, error) {
	if s.n >= len(s.codes) {
		return "", errors.New("generator exhausted")
	}
	code := s.codes[s.n]
	s.n++
	return code, nil
}

// memRepo is a stand-in for the domain repository. A second non-custom
// link with the same destination conflicts, matching the port comment.
// hideDestination makes the first lookup miss so the command must recover
// from a destination conflict.
type memRepo struct {
	byCode          map[string]link.Link
	adds            int
	hideDestination bool
	lookups         int
}

func newMemRepo() *memRepo {
	return &memRepo{byCode: map[string]link.Link{}}
}

func (m *memRepo) Add(_ context.Context, item link.Link) error {
	m.adds++
	code := item.Code().String()
	if _, ok := m.byCode[code]; ok {
		return &link.ConflictError{Reason: "code"}
	}
	if !item.Custom() {
		for _, existing := range m.byCode {
			if !existing.Custom() && existing.Destination().String() == item.Destination().String() {
				return &link.ConflictError{Reason: "destination"}
			}
		}
	}
	m.byCode[code] = item
	m.hideDestination = false
	return nil
}

func (m *memRepo) Get(_ context.Context, code link.Code) (link.Link, error) {
	item, ok := m.byCode[code.String()]
	if !ok {
		return link.Link{}, &link.NotFoundError{}
	}
	return item, nil
}

func (m *memRepo) GetByDestination(_ context.Context, dest link.Destination) (link.Link, error) {
	m.lookups++
	if m.hideDestination && m.lookups == 1 {
		return link.Link{}, &link.NotFoundError{}
	}
	for _, item := range m.byCode {
		if item.Custom() {
			continue
		}
		if item.Destination().String() == dest.String() {
			return item, nil
		}
	}
	return link.Link{}, &link.NotFoundError{}
}

func (m *memRepo) Update(context.Context, link.Code, func(*link.Link) error) error {
	return errors.New("shorten must not update")
}

var _ link.Repository = (*memRepo)(nil)
