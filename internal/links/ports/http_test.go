package ports_test

import (
	"context"
	json "encoding/json/v2"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/common/httpx"
	"github.com/jovrion/veylqo/internal/links/app"
	"github.com/jovrion/veylqo/internal/links/app/command"
	"github.com/jovrion/veylqo/internal/links/app/query"
	"github.com/jovrion/veylqo/internal/links/domain/link"
	"github.com/jovrion/veylqo/internal/links/ports"
)

const secretURL = "https://example.com/docs?token=secret"

func TestSpecHTTPCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		shortenErr error
		getErr     error
		deleteErr  error
		statsErr   error
		want       int
		slug       apperrors.Slug
	}{
		{
			name:       "create incorrect-input",
			method:     http.MethodPost,
			path:       "/api/v1/links",
			body:       `{"url":"` + secretURL + `"}`,
			shortenErr: apperrors.NewIncorrectInput("host is not allowed", nil),
			want:       http.StatusBadRequest,
			slug:       apperrors.IncorrectInput,
		},
		{
			name:       "create unauthorized",
			method:     http.MethodPost,
			path:       "/api/v1/links",
			body:       `{"url":"https://example.com/docs"}`,
			shortenErr: apperrors.NewUnauthorized("publishable key required", nil),
			want:       http.StatusUnauthorized,
			slug:       apperrors.Unauthorized,
		},
		{
			name:   "get not-found",
			method: http.MethodGet,
			path:   "/api/v1/links/Ab3xyZ9",
			getErr: apperrors.NewNotFound("link", nil),
			want:   http.StatusNotFound,
			slug:   apperrors.NotFound,
		},
		{
			name:       "create conflict",
			method:     http.MethodPost,
			path:       "/api/v1/links",
			body:       `{"url":"https://example.com/docs","slug":"taken"}`,
			shortenErr: apperrors.NewConflict("code", nil),
			want:       http.StatusConflict,
			slug:       apperrors.Conflict,
		},
		{
			name:      "delete gone",
			method:    http.MethodDelete,
			path:      "/api/v1/links/Ab3xyZ9",
			deleteErr: apperrors.NewGone("link", nil),
			want:      http.StatusGone,
			slug:      apperrors.Gone,
		},
		{
			name:       "create rate-limited",
			method:     http.MethodPost,
			path:       "/api/v1/links?token=secret",
			body:       `{"url":"https://example.com/docs"}`,
			shortenErr: apperrors.NewRateLimited("slow down", nil),
			want:       http.StatusTooManyRequests,
			slug:       apperrors.RateLimited,
		},
		{
			name:   "get unauthorized",
			method: http.MethodGet,
			path:   "/api/v1/links/Ab3xyZ9",
			getErr: apperrors.NewUnauthorized("admin token required", nil),
			want:   http.StatusUnauthorized,
			slug:   apperrors.Unauthorized,
		},
		{
			name:     "stats rate-limited",
			method:   http.MethodGet,
			path:     "/api/v1/links/Ab3xyZ9/stats",
			statsErr: apperrors.NewRateLimited("slow down", nil),
			want:     http.StatusTooManyRequests,
			slug:     apperrors.RateLimited,
		},
		{
			name:      "delete not-found",
			method:    http.MethodDelete,
			path:      "/api/v1/links/missing1",
			deleteErr: apperrors.NewNotFound("link", nil),
			want:      http.StatusNotFound,
			slug:      apperrors.NotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mux := ports.NewMux(app.Application{
				Commands: app.Commands{
					Shorten: shortenFunc(func(context.Context, command.ShortenLink) (command.ShortenLinkResult, error) {
						return command.ShortenLinkResult{}, tt.shortenErr
					}),
					Delete: deleteFunc(func(context.Context, command.DeleteLink) error {
						return tt.deleteErr
					}),
				},
				Queries: app.Queries{
					Get: getFunc(func(context.Context, query.GetLink) (query.GetLinkResult, error) {
						return query.GetLinkResult{}, tt.getErr
					}),
					Stats: statsFunc(func(context.Context, query.LinkStats) (query.LinkStatsResult, error) {
						return query.LinkStatsResult{}, tt.statsErr
					}),
				},
			})
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, request(tt.method, tt.path, tt.body))
			assertProblem(t, rec, tt.want, tt.slug)
			if strings.Contains(rec.Body.String(), "token=secret") || strings.Contains(rec.Body.String(), "example.com") {
				t.Fatalf("destination leaked into %s", rec.Body.String())
			}
			if rec.Header().Get("Location") != "" {
				t.Fatalf("Location = %q, API errors must not redirect", rec.Header().Get("Location"))
			}
		})
	}
}

func TestCreateLinkMapsRequestAndReturns201(t *testing.T) {
	t.Parallel()

	expires := time.Date(2026, 12, 1, 0, 0, 0, 0, time.FixedZone("CET", 3600))
	var got command.ShortenLink
	mux := ports.NewMux(app.Application{
		Commands: app.Commands{
			Shorten: shortenFunc(func(_ context.Context, cmd command.ShortenLink) (command.ShortenLinkResult, error) {
				got = cmd
				code, err := link.NewCode("Ab3xyZ9")
				if err != nil {
					t.Fatal(err)
				}
				return command.ShortenLinkResult{Code: code}, nil
			}),
		},
	})
	body := `{"url":"` + secretURL + `","slug":"Ab3xyZ9","clean":false,"expires_at":"2026-12-01T00:00:00+01:00"}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, request(http.MethodPost, "/api/v1/links?token=secret", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != httpx.JSONMediaType {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, max-age=0" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if rec.Header().Get("Location") != "" {
		t.Fatal("create must not redirect")
	}
	var resp struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp, json.RejectUnknownMembers(true)); err != nil {
		t.Fatal(err)
	}
	if resp.Code != "Ab3xyZ9" {
		t.Fatalf("code = %q", resp.Code)
	}
	if strings.Contains(rec.Body.String(), "token=secret") || strings.Contains(rec.Body.String(), "example.com") {
		t.Fatalf("destination leaked into %s", rec.Body.String())
	}
	if got.URL != secretURL || got.Slug != "Ab3xyZ9" || !got.SkipClean {
		t.Fatalf("command = %+v", got)
	}
	if !got.ExpiresAt.Equal(expires.UTC()) {
		t.Fatalf("expires_at = %s", got.ExpiresAt)
	}
}

func TestOmittedCleanDoesNotSkipCleaning(t *testing.T) {
	t.Parallel()

	var got command.ShortenLink
	mux := ports.NewMux(app.Application{
		Commands: app.Commands{
			Shorten: shortenFunc(func(_ context.Context, cmd command.ShortenLink) (command.ShortenLinkResult, error) {
				got = cmd
				code, err := link.NewCode("Ab3xyZ9")
				if err != nil {
					return command.ShortenLinkResult{}, err
				}
				return command.ShortenLinkResult{Code: code}, nil
			}),
		},
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, request(http.MethodPost, "/api/v1/links", `{"url":"https://example.com/docs"}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got.SkipClean || !got.ExpiresAt.IsZero() || got.Slug != "" {
		t.Fatalf("command = %+v", got)
	}
}

func TestUnknownFieldIs400AndDoesNotCallShorten(t *testing.T) {
	t.Parallel()

	called := false
	mux := ports.NewMux(app.Application{
		Commands: app.Commands{
			Shorten: shortenFunc(func(context.Context, command.ShortenLink) (command.ShortenLinkResult, error) {
				called = true
				return command.ShortenLinkResult{}, nil
			}),
		},
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, request(http.MethodPost, "/api/v1/links", `{"url":"`+secretURL+`","extra":true}`))
	assertProblem(t, rec, http.StatusBadRequest, apperrors.IncorrectInput)
	if called {
		t.Fatal("shorten was called")
	}
	if strings.Contains(rec.Body.String(), "token=secret") || strings.Contains(rec.Body.String(), "example.com") {
		t.Fatalf("destination leaked into %s", rec.Body.String())
	}
}

func TestGetLinkReturnsStoredRow(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	expires := created.Add(time.Hour)
	code, err := link.NewCode("kept-slug")
	if err != nil {
		t.Fatal(err)
	}
	dest, err := link.NewDestination(secretURL, link.DestinationConfig{})
	if err != nil {
		t.Fatal(err)
	}
	var got query.GetLink
	mux := ports.NewMux(app.Application{
		Queries: app.Queries{
			Get: getFunc(func(_ context.Context, q query.GetLink) (query.GetLinkResult, error) {
				got = q
				return query.GetLinkResult{
					Code:        code,
					Destination: dest,
					Custom:      true,
					CreatedAt:   created,
					ExpiresAt:   expires,
				}, nil
			}),
		},
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, request(http.MethodGet, "/api/v1/links/kept-slug?token=secret", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got.Code != "kept-slug" {
		t.Fatalf("code = %q", got.Code)
	}
	var resp struct {
		Code      string     `json:"code"`
		URL       string     `json:"url"`
		Custom    bool       `json:"custom"`
		CreatedAt time.Time  `json:"created_at"`
		ExpiresAt *time.Time `json:"expires_at"`
		DeletedAt *time.Time `json:"deleted_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp, json.RejectUnknownMembers(true)); err != nil {
		t.Fatal(err)
	}
	if resp.Code != "kept-slug" || !resp.Custom || !resp.CreatedAt.Equal(created) {
		t.Fatalf("response = %+v", resp)
	}
	if resp.URL != dest.String() {
		t.Fatalf("url = %q", resp.URL)
	}
	if resp.ExpiresAt == nil || !resp.ExpiresAt.Equal(expires) || resp.DeletedAt != nil {
		t.Fatalf("times = %+v %+v", resp.ExpiresAt, resp.DeletedAt)
	}
	if strings.Contains(rec.Header().Get("Cache-Control"), "token") || rec.Header().Get("Location") != "" {
		t.Fatal("query or destination leaked into headers")
	}
}

func TestGetOmitsZeroExpiryAndDeletedAt(t *testing.T) {
	t.Parallel()

	code, err := link.NewCode("Ab3xyZ9")
	if err != nil {
		t.Fatal(err)
	}
	dest, err := link.NewDestination("https://example.com/docs", link.DestinationConfig{})
	if err != nil {
		t.Fatal(err)
	}
	mux := ports.NewMux(app.Application{
		Queries: app.Queries{
			Get: getFunc(func(context.Context, query.GetLink) (query.GetLinkResult, error) {
				return query.GetLinkResult{
					Code:        code,
					Destination: dest,
					CreatedAt:   time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
				}, nil
			}),
		},
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, request(http.MethodGet, "/api/v1/links/Ab3xyZ9", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "expires_at") || strings.Contains(rec.Body.String(), "deleted_at") {
		t.Fatalf("zero times were encoded: %s", rec.Body.String())
	}
}

func TestDeleteLinkIs204WithoutBody(t *testing.T) {
	t.Parallel()

	var got command.DeleteLink
	mux := ports.NewMux(app.Application{
		Commands: app.Commands{
			Delete: deleteFunc(func(_ context.Context, cmd command.DeleteLink) error {
				got = cmd
				return nil
			}),
		},
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, request(http.MethodDelete, "/api/v1/links/Ab3xyZ9?token=secret", ""))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, max-age=0" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if rec.Header().Get("Location") != "" {
		t.Fatal("delete must not redirect")
	}
	if got.Code != "Ab3xyZ9" {
		t.Fatalf("code = %q", got.Code)
	}
}

func TestStatsReturnsCount(t *testing.T) {
	t.Parallel()

	var got query.LinkStats
	mux := ports.NewMux(app.Application{
		Queries: app.Queries{
			Stats: statsFunc(func(_ context.Context, q query.LinkStats) (query.LinkStatsResult, error) {
				got = q
				return query.LinkStatsResult{Clicks: 0}, nil
			}),
		},
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, request(http.MethodGet, "/api/v1/links/Ab3xyZ9/stats", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got.Code != "Ab3xyZ9" {
		t.Fatalf("code = %q", got.Code)
	}
	var resp struct {
		Clicks int64 `json:"clicks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp, json.RejectUnknownMembers(true)); err != nil {
		t.Fatal(err)
	}
	if resp.Clicks != 0 {
		t.Fatalf("clicks = %d", resp.Clicks)
	}
	if !strings.Contains(rec.Body.String(), `"clicks":0`) && !strings.Contains(rec.Body.String(), `"clicks": 0`) {
		t.Fatalf("zero count was omitted: %s", rec.Body.String())
	}
}

func TestNonSlugErrorIs500WithoutCause(t *testing.T) {
	t.Parallel()

	mux := ports.NewMux(app.Application{
		Queries: app.Queries{
			Get: getFunc(func(context.Context, query.GetLink) (query.GetLinkResult, error) {
				return query.GetLinkResult{}, errString("store failed for " + secretURL)
			}),
		},
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, request(http.MethodGet, "/api/v1/links/Ab3xyZ9", ""))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != httpx.ProblemMediaType {
		t.Fatalf("Content-Type = %q", got)
	}
	if strings.Contains(rec.Body.String(), "token=secret") || strings.Contains(rec.Body.String(), "store failed") {
		t.Fatalf("cause leaked into %s", rec.Body.String())
	}
}

func TestMissingHandlerIs500(t *testing.T) {
	t.Parallel()

	mux := ports.NewMux(app.Application{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, request(http.MethodPost, "/api/v1/links", `{"url":"https://example.com/docs"}`))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "example.com") {
		t.Fatalf("url leaked into %s", rec.Body.String())
	}
}

func TestRedirectRouteIsNotRegistered(t *testing.T) {
	t.Parallel()

	mux := ports.NewMux(app.Application{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, request(http.MethodGet, "/Ab3xyZ9", ""))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, redirect must not be this mux", rec.Code)
	}
	if rec.Code == http.StatusMovedPermanently || rec.Header().Get("Location") != "" {
		t.Fatal("API mux emitted a redirect")
	}
}

func TestPortsDoesNotImportDatabaseSQL(t *testing.T) {
	t.Parallel()

	root := moduleRoot(t)
	dir := filepath.Join(root, "internal", "links", "ports")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files int
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		files++
		name := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if path == "database/sql" || strings.HasPrefix(path, "database/sql/") {
				pos := fset.Position(spec.Path.Pos())
				t.Errorf("ports imports %s at %s:%d", path, filepath.ToSlash(pos.Filename), pos.Line)
			}
		}
	}
	if files == 0 {
		t.Fatal("no ports files found")
	}
}

func request(method, path, body string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", httpx.JSONMediaType)
	}
	return req
}

func assertProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, slug apperrors.Slug) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, status, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != httpx.ProblemMediaType {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, max-age=0" {
		t.Fatalf("Cache-Control = %q", got)
	}
	var problem struct {
		Type     string `json:"type"`
		Title    string `json:"title"`
		Status   int    `json:"status"`
		Detail   string `json:"detail"`
		Instance string `json:"instance"`
		Slug     string `json:"slug"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem, json.RejectUnknownMembers(true)); err != nil {
		t.Fatal(err)
	}
	if problem.Type != "about:blank" || problem.Status != status || problem.Slug != string(slug) {
		t.Fatalf("problem = %+v", problem)
	}
	if strings.Contains(problem.Instance, "?") {
		t.Fatalf("query leaked into %s", rec.Body.String())
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

type errString string

func (e errString) Error() string { return string(e) }

type shortenFunc func(context.Context, command.ShortenLink) (command.ShortenLinkResult, error)

func (f shortenFunc) Handle(ctx context.Context, cmd command.ShortenLink) (command.ShortenLinkResult, error) {
	return f(ctx, cmd)
}

func (shortenFunc) Name() string { return "ShortenLink" }

type deleteFunc func(context.Context, command.DeleteLink) error

func (f deleteFunc) Handle(ctx context.Context, cmd command.DeleteLink) error {
	return f(ctx, cmd)
}

func (deleteFunc) Name() string { return "DeleteLink" }

type getFunc func(context.Context, query.GetLink) (query.GetLinkResult, error)

func (f getFunc) Handle(ctx context.Context, q query.GetLink) (query.GetLinkResult, error) {
	return f(ctx, q)
}

func (getFunc) Name() string { return "GetLink" }

type statsFunc func(context.Context, query.LinkStats) (query.LinkStatsResult, error)

func (f statsFunc) Handle(ctx context.Context, q query.LinkStats) (query.LinkStatsResult, error) {
	return f(ctx, q)
}

func (statsFunc) Name() string { return "LinkStats" }
