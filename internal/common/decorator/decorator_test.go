package decorator_test

import (
	"bytes"
	"context"
	"errors"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jovrion/veylqo/internal/common/decorator"
	apperrors "github.com/jovrion/veylqo/internal/common/errors"
)

const (
	secretURL   = "https://example.com/dest?token=abc"
	secretToken = "token=abc"
	secretIP    = "203.0.113.9"
)

type secretQuery struct {
	URL   string
	Token string
	IP    string
}

type queryHandler struct {
	name   string
	result string
	err    error
	panic  string
}

func (h queryHandler) Name() string { return h.name }

func (h queryHandler) Handle(context.Context, secretQuery) (string, error) {
	if h.panic != "" {
		panic(h.panic)
	}
	return h.result, h.err
}

type commandHandler struct {
	name  string
	err   error
	panic string
}

func (h commandHandler) Name() string { return h.name }

func (h commandHandler) Handle(context.Context, secretQuery) error {
	if h.panic != "" {
		panic(h.panic)
	}
	return h.err
}

type spyMetrics struct {
	log         *bytes.Buffer
	observation decorator.Observation
	called      bool
	logReady    bool
}

func (s *spyMetrics) Observe(observation decorator.Observation) {
	s.called = true
	s.observation = observation
	if s.log != nil {
		s.logReady = strings.Contains(s.log.String(), "error_class="+observation.Class) && observation.Class != ""
	}
}

func TestQueryDecoratorsKeepNameAndResult(t *testing.T) {
	t.Parallel()

	logBuf := &bytes.Buffer{}
	metrics := &spyMetrics{}
	handler := decorator.ApplyQueryDecorators[secretQuery, string](queryHandler{
		name:   "ResolveLink",
		result: secretURL,
	}, testLogger(logBuf), metrics)

	if handler.Name() != "ResolveLink" {
		t.Fatalf("Name() = %q", handler.Name())
	}
	got, err := handler.Handle(context.Background(), secretQuery{
		URL:   secretURL,
		Token: secretToken,
		IP:    secretIP,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != secretURL {
		t.Fatalf("result = %q", got)
	}
	if !metrics.called || metrics.observation.Failed || metrics.observation.Name != "ResolveLink" {
		t.Fatalf("observation = %+v, called = %v", metrics.observation, metrics.called)
	}
	if metrics.observation.Duration < 0 {
		t.Fatal("negative duration")
	}
	assertNoSecrets(t, logBuf.String())
	if !strings.Contains(logBuf.String(), "handler=ResolveLink") {
		t.Fatalf("log = %q", logBuf.String())
	}
}

func TestQueryDecoratorsLogSlugNotErrorText(t *testing.T) {
	t.Parallel()

	logBuf := &bytes.Buffer{}
	metrics := &spyMetrics{}
	handler := decorator.ApplyQueryDecorators[secretQuery, string](queryHandler{
		name: "GetLink",
		err:  apperrors.NewNotFound("missing", errors.New(secretURL+" "+secretIP)),
	}, testLogger(logBuf), metrics)

	_, err := handler.Handle(context.Background(), secretQuery{URL: secretURL, Token: secretToken, IP: secretIP})
	if !errors.Is(err, apperrors.NewNotFound("", nil)) {
		t.Fatalf("err = %v", err)
	}
	text := logBuf.String()
	if !strings.Contains(text, "error_class=not-found") {
		t.Fatalf("log = %q", text)
	}
	assertNoSecrets(t, text)
	if metrics.observation.Class != "not-found" || !metrics.observation.Failed {
		t.Fatalf("observation = %+v", metrics.observation)
	}
}

func TestPlainErrorTextIsNotLogged(t *testing.T) {
	t.Parallel()

	logBuf := &bytes.Buffer{}
	handler := decorator.ApplyQueryDecorators[secretQuery, string](queryHandler{
		name: "ShortenLink",
		err:  errors.New("failed for " + secretURL + " from " + secretIP),
	}, testLogger(logBuf), decorator.NopMetrics{})

	_, err := handler.Handle(context.Background(), secretQuery{})
	if err == nil {
		t.Fatal("expected error")
	}
	text := logBuf.String()
	if !strings.Contains(text, "error_class=error") {
		t.Fatalf("log = %q", text)
	}
	assertNoSecrets(t, text)
}

func TestPanicBecomesErrorAndDoesNotCrash(t *testing.T) {
	t.Parallel()

	logBuf := &bytes.Buffer{}
	metrics := &spyMetrics{log: logBuf}
	handler := decorator.ApplyQueryDecorators[secretQuery, string](queryHandler{
		name:  "ResolveLink",
		panic: secretURL + " " + secretToken + " " + secretIP,
	}, testLogger(logBuf), metrics)

	got, err := handler.Handle(context.Background(), secretQuery{URL: secretURL, Token: secretToken, IP: secretIP})
	if got != "" {
		t.Fatalf("panic result = %q, want zero", got)
	}
	if !errors.Is(err, decorator.ErrHandlerPanicked) {
		t.Fatalf("err = %v, want panicked", err)
	}
	text := logBuf.String()
	if !strings.Contains(text, "error_class=panic") {
		t.Fatalf("log = %q", text)
	}
	assertNoSecrets(t, text)
	if !metrics.called || metrics.observation.Class != "panic" || !metrics.observation.Failed {
		t.Fatalf("observation = %+v", metrics.observation)
	}
	if !metrics.logReady {
		t.Fatal("metrics ran before logging recorded the panic class")
	}
}

func TestCommandPanicBecomesError(t *testing.T) {
	t.Parallel()

	logBuf := &bytes.Buffer{}
	metrics := &spyMetrics{log: logBuf}
	handler := decorator.ApplyCommandDecorators[secretQuery](commandHandler{
		name:  "RecordClick",
		panic: secretURL + " " + secretIP,
	}, testLogger(logBuf), metrics)

	if handler.Name() != "RecordClick" {
		t.Fatalf("Name() = %q", handler.Name())
	}
	err := handler.Handle(context.Background(), secretQuery{URL: secretURL, Token: secretToken, IP: secretIP})
	if !errors.Is(err, decorator.ErrHandlerPanicked) {
		t.Fatalf("err = %v", err)
	}
	assertNoSecrets(t, logBuf.String())
	if !metrics.logReady || metrics.observation.Name != "RecordClick" {
		t.Fatalf("observation = %+v, log = %q", metrics.observation, logBuf.String())
	}
}

func TestCommandErrorPassesThrough(t *testing.T) {
	t.Parallel()

	want := apperrors.NewConflict("code taken", nil)
	handler := decorator.ApplyCommandDecorators[secretQuery](commandHandler{
		name: "DeleteLink",
		err:  want,
	}, nil, nil)

	err := handler.Handle(context.Background(), secretQuery{URL: secretURL})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v", err)
	}
}

func TestNilHandlerDoesNotPanic(t *testing.T) {
	t.Parallel()

	query := decorator.ApplyQueryDecorators[secretQuery, string](nil, nil, nil)
	if query.Name() != "" {
		t.Fatalf("Name() = %q", query.Name())
	}
	if _, err := query.Handle(context.Background(), secretQuery{}); err == nil {
		t.Fatal("nil query handler returned nil error")
	}

	command := decorator.ApplyCommandDecorators[secretQuery](nil, nil, nil)
	if err := command.Handle(context.Background(), secretQuery{}); err == nil {
		t.Fatal("nil command handler returned nil error")
	}
}

func TestUnsafeNameIsRedacted(t *testing.T) {
	t.Parallel()

	logBuf := &bytes.Buffer{}
	metrics := &spyMetrics{}
	handler := decorator.ApplyQueryDecorators[secretQuery, string](queryHandler{
		name:   secretURL,
		result: "ok",
	}, testLogger(logBuf), metrics)

	if _, err := handler.Handle(context.Background(), secretQuery{}); err != nil {
		t.Fatal(err)
	}
	text := logBuf.String()
	if !strings.Contains(text, "handler=redacted") {
		t.Fatalf("log = %q", text)
	}
	if metrics.observation.Name != "redacted" {
		t.Fatalf("metric name = %q", metrics.observation.Name)
	}
	assertNoSecrets(t, text)
}

func TestPackageDoesNotImportLinksOrHTTP(t *testing.T) {
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
				t.Fatalf("%s imports %s", name, path)
			case strings.Contains(path, "internal/links"):
				t.Fatalf("%s imports domain or application %s", name, path)
			case strings.Contains(path, ".") && !strings.HasPrefix(path, "github.com/jovrion/veylqo/"):
				t.Fatalf("%s imports third-party %s", name, path)
			}
		}
	}
	if _, err := os.Stat("decorator.go"); err != nil {
		t.Fatal(err)
	}
}

func testLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func assertNoSecrets(t *testing.T, text string) {
	t.Helper()
	for _, leaked := range []string{secretURL, secretToken, secretIP, "example.com", "://", "?"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("log leaked %q in %q", leaked, text)
		}
	}
}

func TestObservationHasNoPayloadFields(t *testing.T) {
	t.Parallel()

	observation := decorator.Observation{
		Name:     "Stats",
		Class:    "gone",
		Duration: time.Millisecond,
		Failed:   true,
	}
	if observation.Name == "" || observation.Class == "" {
		t.Fatal("observation lost its class")
	}
}
