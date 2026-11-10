package errors

import (
	stderrors "errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEachDocumentedSlugHasAConstructor(t *testing.T) {
	t.Parallel()

	documented := []Slug{
		IncorrectInput,
		NotFound,
		Gone,
		Conflict,
		Unauthorized,
		RateLimited,
	}
	constructors := map[Slug]func(string, error) *SlugError{
		IncorrectInput: NewIncorrectInput,
		NotFound:       NewNotFound,
		Gone:           NewGone,
		Conflict:       NewConflict,
		Unauthorized:   NewUnauthorized,
		RateLimited:    NewRateLimited,
	}
	if len(constructors) != len(documented) {
		t.Fatalf("constructors = %d, documented slugs = %d", len(constructors), len(documented))
	}

	seen := map[Slug]bool{}
	for _, slug := range documented {
		if seen[slug] {
			t.Fatalf("duplicate documented slug %q", slug)
		}
		seen[slug] = true
		if slug == "" || strings.ContainsAny(string(slug), " \t\r\n") {
			t.Fatalf("slug %q is not a stable token", slug)
		}
		ctor, ok := constructors[slug]
		if !ok {
			t.Fatalf("slug %q has no constructor", slug)
		}
		err := ctor("detail", nil)
		if err == nil {
			t.Fatalf("slug %q constructor returned nil", slug)
		}
		if err.Slug() != slug {
			t.Fatalf("Slug() = %q, want %q", err.Slug(), slug)
		}
		if !strings.Contains(err.Error(), string(slug)) {
			t.Fatalf("Error() = %q, want slug %q", err.Error(), slug)
		}
		if !stderrors.Is(err, ctor("other detail", stderrors.New("cause"))) {
			t.Fatalf("errors.Is did not match slug %q", slug)
		}
	}
}

func TestWrapKeepsSlugAndFindsCause(t *testing.T) {
	t.Parallel()

	cause := stderrors.New("https://example.com/secret?token=abc")
	inner := NewNotFound("link not found", cause)
	outer := NewConflict("code taken", inner)

	if outer.Slug() != Conflict {
		t.Fatalf("outer slug = %q", outer.Slug())
	}
	if !stderrors.Is(outer, cause) {
		t.Fatal("wrapped cause was not found")
	}
	if !stderrors.Is(outer, inner) {
		t.Fatal("wrapped slug error was not found")
	}
	if stderrors.Is(outer, NewGone("expired", nil)) {
		t.Fatal("conflict matched gone")
	}

	got, ok := As(outer)
	if !ok || got.Slug() != Conflict {
		t.Fatalf("As = (%v, %v), want conflict", got, ok)
	}
	if got.Unwrap() != inner {
		t.Fatal("Unwrap did not return the inner slug error")
	}
	if inner.Unwrap() != cause {
		t.Fatal("Unwrap did not return the cause")
	}

	text := outer.Error() + " " + inner.Error()
	for _, leaked := range []string{"https://", "token", "example.com", "?"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("Error() leaked %q in %q", leaked, text)
		}
	}
}

func TestMessageOmitsURLQueryAndNewline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message string
		want    string
	}{
		{name: "empty", message: "  ", want: string(IncorrectInput)},
		{name: "plain", message: "host is not allowed", want: "incorrect-input: host is not allowed"},
		{name: "url", message: "https://example.com/a", want: "incorrect-input: detail omitted"},
		{name: "query", message: "bad?token=1", want: "incorrect-input: detail omitted"},
		{name: "newline", message: "line\nnext", want: "incorrect-input: detail omitted"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := NewIncorrectInput(tt.message, nil)
			if err.Error() != tt.want {
				t.Fatalf("Error() = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestNilReceiver(t *testing.T) {
	t.Parallel()

	var err *SlugError
	if err.Error() != "application error" {
		t.Fatalf("Error() = %q", err.Error())
	}
	if err.Slug() != "" {
		t.Fatalf("Slug() = %q", err.Slug())
	}
	if err.Unwrap() != nil {
		t.Fatal("Unwrap() was not nil")
	}
	if err.Is(NewNotFound("x", nil)) {
		t.Fatal("nil Is matched")
	}
	if _, ok := As(nil); ok {
		t.Fatal("As(nil) matched")
	}
}

func TestPackageDoesNotImportHTTP(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	matches, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, name := range matches {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, name)
	}
	if len(files) == 0 {
		t.Fatal("no production files")
	}
	for _, name := range files {
		file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			switch {
			case path == "net/http", path == "database/sql":
				t.Fatalf("%s imports %s; HTTP translation belongs in ports", name, path)
			case strings.Contains(path, ".") && !strings.HasPrefix(path, "github.com/jovrion/veylqo/"):
				t.Fatalf("%s imports third-party %s", name, path)
			}
		}
	}
	if _, err := os.Stat("errors.go"); err != nil {
		t.Fatal(err)
	}
}
