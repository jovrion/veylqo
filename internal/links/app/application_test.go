package app_test

import (
	"bytes"
	"context"
	"errors"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jovrion/veylqo/internal/common/decorator"
	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/links/app"
	"github.com/jovrion/veylqo/internal/links/app/command"
	"github.com/jovrion/veylqo/internal/links/app/query"
	"github.com/jovrion/veylqo/internal/links/domain/link"
)

const secretURL = "https://example.com/a?token=secret"

func TestApplicationWiresCommandsAndQueries(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	clicks := &memClicks{}
	logBuf := &bytes.Buffer{}
	metrics := &spyMetrics{}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	application := newTestApplication(t, repo, clicks, logBuf, metrics, now)

	if application.Commands.Shorten.Name() != "ShortenLink" ||
		application.Commands.RecordClick.Name() != "RecordClick" ||
		application.Commands.Delete.Name() != "DeleteLink" ||
		application.Queries.Resolve.Name() != "ResolveLink" ||
		application.Queries.Get.Name() != "GetLink" ||
		application.Queries.Stats.Name() != "LinkStats" {
		t.Fatalf("names = %s %s %s %s %s %s",
			application.Commands.Shorten.Name(),
			application.Commands.RecordClick.Name(),
			application.Commands.Delete.Name(),
			application.Queries.Resolve.Name(),
			application.Queries.Get.Name(),
			application.Queries.Stats.Name(),
		)
	}

	ctx := context.Background()
	shortened, err := application.Commands.Shorten.Handle(ctx, command.ShortenLink{URL: secretURL})
	if err != nil {
		t.Fatal(err)
	}
	if shortened.Code.String() != "Ab3xyZ9" {
		t.Fatalf("code = %s", shortened.Code.String())
	}

	resolved, err := application.Queries.Resolve.Handle(ctx, query.ResolveLink{Code: "Ab3xyZ9"})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Destination.String() != secretURL {
		t.Fatalf("destination = %s", resolved.Destination.String())
	}
	if clicks.enqueued != 0 {
		t.Fatal("resolve recorded a click")
	}

	got, err := application.Queries.Get.Handle(ctx, query.GetLink{Code: "Ab3xyZ9"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Code.String() != "Ab3xyZ9" || got.Destination.String() != secretURL {
		t.Fatalf("get = %+v", got)
	}

	stats, err := application.Queries.Stats.Handle(ctx, query.LinkStats{Code: "Ab3xyZ9"})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Clicks != 0 {
		t.Fatalf("clicks = %d", stats.Clicks)
	}

	if err := application.Commands.RecordClick.Handle(ctx, command.RecordClick{Code: "Ab3xyZ9"}); err != nil {
		t.Fatal(err)
	}
	stats, err = application.Queries.Stats.Handle(ctx, query.LinkStats{Code: "Ab3xyZ9"})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Clicks != 1 || clicks.enqueued != 1 {
		t.Fatalf("clicks = %d, enqueued = %d", stats.Clicks, clicks.enqueued)
	}

	if err := application.Commands.Delete.Handle(ctx, command.DeleteLink{Code: "Ab3xyZ9"}); err != nil {
		t.Fatal(err)
	}
	_, err = application.Queries.Resolve.Handle(ctx, query.ResolveLink{Code: "Ab3xyZ9"})
	assertSlug(t, err, apperrors.Gone)

	want := []string{"ShortenLink", "ResolveLink", "GetLink", "LinkStats", "RecordClick", "LinkStats", "DeleteLink", "ResolveLink"}
	if strings.Join(metrics.names, ",") != strings.Join(want, ",") {
		t.Fatalf("observed %v, want %v", metrics.names, want)
	}
	if strings.Contains(logBuf.String(), secretURL) || strings.Contains(logBuf.String(), "token=secret") {
		t.Fatalf("log leaked destination: %s", logBuf.String())
	}
	if !strings.Contains(logBuf.String(), "handler=ShortenLink") {
		t.Fatalf("log = %s", logBuf.String())
	}
}

func TestHandlersLeaveWrapped(t *testing.T) {
	t.Parallel()

	repo := &panicRepo{panicMsg: secretURL + " 203.0.113.9"}
	clicks := &memClicks{panicMsg: secretURL}
	logBuf := &bytes.Buffer{}
	metrics := &spyMetrics{}
	application := newTestApplication(t, repo, clicks, logBuf, metrics, time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC))

	_, err := application.Queries.Resolve.Handle(context.Background(), query.ResolveLink{Code: "Ab3xyZ9"})
	if !errors.Is(err, decorator.ErrHandlerPanicked) {
		t.Fatalf("resolve panic = %v", err)
	}
	if err := application.Commands.RecordClick.Handle(context.Background(), command.RecordClick{Code: "Ab3xyZ9"}); !errors.Is(err, decorator.ErrHandlerPanicked) {
		t.Fatalf("record panic = %v", err)
	}
	if len(metrics.classes) != 2 || metrics.classes[0] != "panic" || metrics.classes[1] != "panic" {
		t.Fatalf("classes = %v", metrics.classes)
	}
	if strings.Join(metrics.names, ",") != "ResolveLink,RecordClick" {
		t.Fatalf("names = %v", metrics.names)
	}
	logged := logBuf.String()
	if strings.Contains(logged, secretURL) || strings.Contains(logged, "token=secret") || strings.Contains(logged, "203.0.113.9") {
		t.Fatalf("log leaked panic payload: %s", logged)
	}
	if !strings.Contains(logged, "error_class=panic") {
		t.Fatalf("log = %s", logged)
	}
}

func TestNewApplicationDoesNotPanicOnNilDependencies(t *testing.T) {
	t.Parallel()

	application := app.NewApplication(app.Dependencies{})
	if application.Commands.Shorten == nil || application.Commands.RecordClick == nil ||
		application.Commands.Delete == nil || application.Queries.Resolve == nil ||
		application.Queries.Get == nil || application.Queries.Stats == nil {
		t.Fatal("a handler left nil")
	}
	if _, err := application.Commands.Shorten.Handle(context.Background(), command.ShortenLink{URL: secretURL}); err == nil {
		t.Fatal("nil repository succeeded")
	}
}

func TestAppDoesNotImportHTTPOrSQL(t *testing.T) {
	t.Parallel()

	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files int
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" || strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		files++
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(fset, name, body, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rootFile := filepath.Dir(path) == root
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			switch {
			case importPath == "net/http" || strings.HasPrefix(importPath, "net/http/"):
				t.Errorf("%s imports %s", path, importPath)
			case importPath == "database/sql" || strings.HasPrefix(importPath, "database/sql/"):
				t.Errorf("%s imports %s", path, importPath)
			case rootFile && (strings.Contains(importPath, "/internal/links/adapters") ||
				strings.Contains(importPath, "/internal/links/ports") ||
				strings.Contains(importPath, "/internal/links/service")):
				t.Errorf("%s wires %s", path, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 {
		t.Fatal("no app files found")
	}
}

func newTestApplication(t *testing.T, links link.Repository, clicks *memClicks, logBuf *bytes.Buffer, metrics decorator.Metrics, now time.Time) app.Application {
	t.Helper()
	allow, err := command.NewAllowlist([]string{"example.com"})
	if err != nil {
		t.Fatal(err)
	}
	return app.NewApplication(app.Dependencies{
		Links:    links,
		Clicks:   clicks,
		Recorder: clicks,
		Codes:    &seqCodes{codes: []string{"Ab3xyZ9"}},
		Cleaner:  link.DefaultCleaner(),
		Allow:    allow,
		Now:      func() time.Time { return now },
		Logger:   slog.New(slog.NewTextHandler(logBuf, nil)),
		Metrics:  metrics,
	})
}

func assertSlug(t *testing.T, err error, want apperrors.Slug) {
	t.Helper()
	slug, ok := apperrors.As(err)
	if !ok || slug.Slug() != want {
		t.Fatalf("err = %v, want slug %s", err, want)
	}
}

type spyMetrics struct {
	names   []string
	classes []string
}

func (s *spyMetrics) Observe(observation decorator.Observation) {
	s.names = append(s.names, observation.Name)
	s.classes = append(s.classes, observation.Class)
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

type memRepo struct {
	byCode map[string]link.Link
}

func newMemRepo() *memRepo {
	return &memRepo{byCode: map[string]link.Link{}}
}

func (m *memRepo) Add(_ context.Context, item link.Link) error {
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

func (m *memRepo) Update(_ context.Context, code link.Code, updateFn func(*link.Link) error) error {
	if updateFn == nil {
		return errors.New("nil update")
	}
	item, ok := m.byCode[code.String()]
	if !ok {
		return &link.NotFoundError{}
	}
	if err := updateFn(&item); err != nil {
		return err
	}
	m.byCode[code.String()] = item
	return nil
}

type panicRepo struct {
	panicMsg string
}

func (p *panicRepo) Add(context.Context, link.Link) error {
	panic(p.panicMsg)
}

func (p *panicRepo) Get(context.Context, link.Code) (link.Link, error) {
	panic(p.panicMsg)
}

func (p *panicRepo) GetByDestination(context.Context, link.Destination) (link.Link, error) {
	panic(p.panicMsg)
}

func (p *panicRepo) Update(context.Context, link.Code, func(*link.Link) error) error {
	panic(p.panicMsg)
}

type memClicks struct {
	enqueued int
	panicMsg string
}

func (m *memClicks) Enqueue(context.Context, link.Code) error {
	if m.panicMsg != "" {
		panic(m.panicMsg)
	}
	m.enqueued++
	return nil
}

func (m *memClicks) Count(context.Context, link.Code) (int64, error) {
	return int64(m.enqueued), nil
}

var (
	_ link.Repository       = (*memRepo)(nil)
	_ link.Repository       = (*panicRepo)(nil)
	_ command.ClickRecorder = (*memClicks)(nil)
	_ app.ClickCounter      = (*memClicks)(nil)
	_ command.CodeGenerator = (*seqCodes)(nil)
)
