package ports_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/common/httpx"
	"github.com/jovrion/veylqo/internal/links/adapters"
	"github.com/jovrion/veylqo/internal/links/app"
	"github.com/jovrion/veylqo/internal/links/app/command"
	"github.com/jovrion/veylqo/internal/links/app/query"
	"github.com/jovrion/veylqo/internal/links/domain/link"
	"github.com/jovrion/veylqo/internal/links/ports"
)

func TestRedirectReturns302AndLocation(t *testing.T) {
	t.Parallel()

	if http.StatusFound != 302 {
		t.Fatal("StatusFound is not 302")
	}
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	repo := adapters.NewMemoryRepository()
	item, err := link.NewLink(link.LinkSpec{
		Code:        "Ab3xyZ9",
		Destination: secretURL,
		Now:         now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Add(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	recorded := make(chan command.RecordClick, 1)
	application := app.NewApplication(app.Dependencies{
		Links: repo,
		Recorder: recordEnqueue(func(_ context.Context, code link.Code) error {
			recorded <- command.RecordClick{Code: code.String()}
			return nil
		}),
		Now: func() time.Time { return now },
	})
	mux := ports.NewRedirectMux(application)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, request(http.MethodGet, "/Ab3xyZ9?token=secret", ""))

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Code == http.StatusMovedPermanently || rec.Code == http.StatusTemporaryRedirect || rec.Code == http.StatusPermanentRedirect {
		t.Fatalf("status = %d, redirect must be 302", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != item.Destination().String() {
		t.Fatalf("Location = %q, want %q", got, item.Destination().String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, max-age=0" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "token=secret") || strings.Contains(rec.Body.String(), "example.com") {
		t.Fatalf("destination leaked into body %s", rec.Body.String())
	}
	select {
	case got := <-recorded:
		if got.Code != "Ab3xyZ9" {
			t.Fatalf("RecordClick code = %q", got.Code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RecordClick was not fired")
	}
}

func TestRedirectExpiredIs410AndMissingIs404(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	repo := adapters.NewMemoryRepository()
	expired, err := link.NewLink(link.LinkSpec{
		Code:        "Ab3xyZ9",
		Destination: secretURL,
		Now:         now.Add(-2 * time.Hour),
		ExpiresAt:   now.Add(-time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Add(context.Background(), expired); err != nil {
		t.Fatal(err)
	}
	deleted, err := link.NewLink(link.LinkSpec{
		Code:        "kept-ok",
		Destination: secretURL,
		Custom:      true,
		Now:         now.Add(-time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := deleted.Delete(now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Add(context.Background(), deleted); err != nil {
		t.Fatal(err)
	}
	var clicks atomic.Int64
	application := app.NewApplication(app.Dependencies{
		Links: repo,
		Recorder: recordEnqueue(func(context.Context, link.Code) error {
			clicks.Add(1)
			return nil
		}),
		Now: func() time.Time { return now },
	})
	mux := ports.NewRedirectMux(application)

	tests := []struct {
		name string
		path string
		want int
		slug apperrors.Slug
	}{
		{name: "expired", path: "/Ab3xyZ9?token=secret", want: http.StatusGone, slug: apperrors.Gone},
		{name: "deleted", path: "/kept-ok", want: http.StatusGone, slug: apperrors.Gone},
		{name: "missing", path: "/missing1", want: http.StatusNotFound, slug: apperrors.NotFound},
		{name: "uuid", path: "/018f4d4a-7c1e-7b2a-8c3d-9e0f1a2b3c4d", want: http.StatusNotFound, slug: apperrors.NotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, request(http.MethodGet, tt.path, ""))
			assertProblem(t, rec, tt.want, tt.slug)
			if rec.Code == http.StatusMovedPermanently || rec.Header().Get("Location") != "" {
				t.Fatalf("status = %d, Location = %q", rec.Code, rec.Header().Get("Location"))
			}
			if strings.Contains(rec.Body.String(), "token=secret") || strings.Contains(rec.Body.String(), "example.com") {
				t.Fatalf("destination leaked into %s", rec.Body.String())
			}
		})
	}
	if clicks.Load() != 0 {
		t.Fatalf("RecordClick calls = %d, want 0", clicks.Load())
	}
}

func TestRedirectDoesNotBlockOnRecordClick(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	dest, err := link.NewDestination(secretURL, link.DestinationConfig{})
	if err != nil {
		t.Fatal(err)
	}

	mux := ports.NewRedirectMux(app.Application{
		Queries: app.Queries{
			Resolve: resolveFunc(func(context.Context, query.ResolveLink) (query.ResolveLinkResult, error) {
				return query.ResolveLinkResult{Destination: dest}, nil
			}),
		},
		Commands: app.Commands{
			RecordClick: recordFunc(func(context.Context, command.RecordClick) error {
				close(started)
				<-release
				return errString("click store failed for " + secretURL)
			}),
		},
	})
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		mux.ServeHTTP(rec, request(http.MethodGet, "/Ab3xyZ9", ""))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("GET /{code} blocked on RecordClick")
	}
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Code == http.StatusMovedPermanently {
		t.Fatal("status is 301")
	}
	if got := rec.Header().Get("Location"); got != dest.String() {
		t.Fatalf("Location = %q, want %q", got, dest.String())
	}
	if strings.Contains(rec.Body.String(), "token=secret") {
		t.Fatalf("destination leaked into body %s", rec.Body.String())
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("RecordClick was not fired")
	}
	once.Do(func() { close(release) })
}

func TestReservedRoutesDoNotRedirect(t *testing.T) {
	t.Parallel()

	var resolved bool
	mux := ports.NewRedirectMux(app.Application{
		Queries: app.Queries{
			Resolve: resolveFunc(func(context.Context, query.ResolveLink) (query.ResolveLinkResult, error) {
				resolved = true
				return query.ResolveLinkResult{}, apperrors.NewNotFound("link", nil)
			}),
		},
		Commands: app.Commands{
			RecordClick: recordFunc(func(context.Context, command.RecordClick) error {
				t.Error("RecordClick ran for a reserved route")
				return nil
			}),
		},
	})
	for _, path := range []string{"/api", "/api/", "/healthz", "/readyz", "/metrics", "/healthz/"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, request(http.MethodGet, path, ""))
		if rec.Code == http.StatusFound || rec.Code == http.StatusMovedPermanently || rec.Code == http.StatusTemporaryRedirect || rec.Code == http.StatusPermanentRedirect {
			t.Fatalf("%s status = %d", path, rec.Code)
		}
		if rec.Header().Get("Location") != "" {
			t.Fatalf("%s Location = %q", path, rec.Header().Get("Location"))
		}
	}
	if resolved {
		t.Fatal("reserved route called Resolve")
	}
}

func TestParentMuxReservedRouteWinsOverRedirect(t *testing.T) {
	t.Parallel()

	var resolved bool
	redirect := ports.NewRedirectMux(app.Application{
		Queries: app.Queries{
			Resolve: resolveFunc(func(_ context.Context, q query.ResolveLink) (query.ResolveLinkResult, error) {
				resolved = true
				if q.Code != "Ab3xyZ9" {
					t.Errorf("code = %q", q.Code)
				}
				dest, err := link.NewDestination("https://example.com/docs", link.DestinationConfig{})
				if err != nil {
					return query.ResolveLinkResult{}, err
				}
				return query.ResolveLinkResult{Destination: dest}, nil
			}),
		},
	})
	parent := http.NewServeMux()
	parent.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	parent.Handle("/", redirect)

	health := httptest.NewRecorder()
	parent.ServeHTTP(health, request(http.MethodGet, "/healthz", ""))
	if health.Code != http.StatusNoContent || health.Header().Get("Location") != "" {
		t.Fatalf("health status = %d, Location = %q", health.Code, health.Header().Get("Location"))
	}
	if resolved {
		t.Fatal("healthz called Resolve")
	}

	rec := httptest.NewRecorder()
	parent.ServeHTTP(rec, request(http.MethodGet, "/Ab3xyZ9", ""))
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Code == http.StatusMovedPermanently {
		t.Fatal("status is 301")
	}
	if got := rec.Header().Get("Location"); got != "https://example.com/docs" {
		t.Fatalf("Location = %q", got)
	}
	if !resolved {
		t.Fatal("redirect was not reached")
	}
}

func TestRedirectNilResolveIs500(t *testing.T) {
	t.Parallel()

	mux := ports.NewRedirectMux(app.Application{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, request(http.MethodGet, "/Ab3xyZ9?token=secret", ""))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != httpx.ProblemMediaType {
		t.Fatalf("Content-Type = %q", got)
	}
	if rec.Header().Get("Location") != "" || rec.Code == http.StatusMovedPermanently {
		t.Fatal("nil resolve redirected")
	}
	if strings.Contains(rec.Body.String(), "token=secret") || strings.Contains(rec.Body.String(), "example.com") {
		t.Fatalf("query leaked into %s", rec.Body.String())
	}
}

type resolveFunc func(context.Context, query.ResolveLink) (query.ResolveLinkResult, error)

func (f resolveFunc) Handle(ctx context.Context, q query.ResolveLink) (query.ResolveLinkResult, error) {
	return f(ctx, q)
}

func (resolveFunc) Name() string { return "ResolveLink" }

type recordFunc func(context.Context, command.RecordClick) error

func (f recordFunc) Handle(ctx context.Context, cmd command.RecordClick) error {
	return f(ctx, cmd)
}

func (recordFunc) Name() string { return "RecordClick" }

type recordEnqueue func(context.Context, link.Code) error

func (f recordEnqueue) Enqueue(ctx context.Context, code link.Code) error {
	return f(ctx, code)
}
