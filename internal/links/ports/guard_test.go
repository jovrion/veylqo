package ports_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/links/app"
	"github.com/jovrion/veylqo/internal/links/app/command"
	"github.com/jovrion/veylqo/internal/links/app/query"
	"github.com/jovrion/veylqo/internal/links/domain/link"
	"github.com/jovrion/veylqo/internal/links/ports"
)

const (
	publishableKey = "pub-key-secret"
	adminToken     = "admin-token-secret"
	allowedOrigin  = "https://app.example"
)

func TestPostWithoutPublishableKeyIs401(t *testing.T) {
	t.Parallel()

	var called bool
	handler := apiHandler(t, func() { called = true })
	rec := httptest.NewRecorder()
	req := request(http.MethodPost, "/api/v1/links?token=secret", `{"url":"https://example.com/docs"}`)
	handler.ServeHTTP(rec, req)

	assertProblem(t, rec, http.StatusUnauthorized, apperrors.Unauthorized)
	if called {
		t.Fatal("shorten ran without a publishable key")
	}
	if rec.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", rec.Header().Get("WWW-Authenticate"))
	}
	if strings.Contains(rec.Body.String(), "token=secret") || strings.Contains(rec.Body.String(), "example.com") || strings.Contains(rec.Body.String(), publishableKey) {
		t.Fatalf("secret leaked into %s", rec.Body.String())
	}
	if rec.Header().Get("Location") != "" || rec.Code == http.StatusMovedPermanently {
		t.Fatal("unauthorized response redirected")
	}
}

func TestWrongCredentialDoesNotCrossRoutes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		header string
	}{
		{name: "missing bearer", method: http.MethodPost, path: "/api/v1/links"},
		{name: "wrong key", method: http.MethodPost, path: "/api/v1/links", header: "Bearer wrong-key"},
		{name: "admin token cannot shorten", method: http.MethodPost, path: "/api/v1/links", header: "Bearer " + adminToken},
		{name: "query key is not a credential", method: http.MethodPost, path: "/api/v1/links?key=" + publishableKey},
		{name: "publishable key cannot delete", method: http.MethodDelete, path: "/api/v1/links/Ab3xyZ9", header: "Bearer " + publishableKey},
		{name: "publishable key cannot read", method: http.MethodGet, path: "/api/v1/links/Ab3xyZ9", header: "Bearer " + publishableKey},
		{name: "publishable key cannot read stats", method: http.MethodGet, path: "/api/v1/links/Ab3xyZ9/stats", header: "Bearer " + publishableKey},
		{name: "missing admin token", method: http.MethodDelete, path: "/api/v1/links/Ab3xyZ9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var called bool
			handler := apiHandler(t, func() { called = true })
			req := request(tt.method, tt.path, `{"url":"https://example.com/docs"}`)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			assertProblem(t, rec, http.StatusUnauthorized, apperrors.Unauthorized)
			if called {
				t.Fatal("handler ran")
			}
			if strings.Contains(rec.Body.String(), publishableKey) || strings.Contains(rec.Body.String(), adminToken) || strings.Contains(rec.Body.String(), "example.com") {
				t.Fatalf("secret leaked into %s", rec.Body.String())
			}
		})
	}
}

func TestPublishableKeyReachesShorten(t *testing.T) {
	t.Parallel()

	var called bool
	handler := apiHandler(t, func() { called = true })
	req := request(http.MethodPost, "/api/v1/links", `{"url":"https://example.com/docs"}`)
	req.Header.Set("Authorization", "bearer "+publishableKey)
	req.Header.Set("Origin", allowedOrigin)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Fatal("shorten was not called")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Fatalf("Allow-Origin = %q", got)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") == "*" {
		t.Fatal("wildcard CORS")
	}
	if rec.Code == http.StatusMovedPermanently {
		t.Fatal("shorten redirected")
	}
}

func TestAdminTokenReachesGetDeleteAndStats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method string
		path   string
		want   int
	}{
		{method: http.MethodGet, path: "/api/v1/links/Ab3xyZ9", want: http.StatusOK},
		{method: http.MethodDelete, path: "/api/v1/links/Ab3xyZ9", want: http.StatusNoContent},
		{method: http.MethodGet, path: "/api/v1/links/Ab3xyZ9/stats", want: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			handler := apiHandler(t, func() {})
			req := request(tt.method, tt.path, "")
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Sec-Fetch-Site", "same-origin")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRedirectDoesNotRequirePublishableKey(t *testing.T) {
	t.Parallel()

	var called bool
	guard := newGuard(t, ports.LimitConfig{Burst: 5, Refill: time.Second})
	handler := guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/Ab3xyZ9" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Location", "https://example.com/docs")
		w.WriteHeader(http.StatusFound)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, request(http.MethodGet, "/Ab3xyZ9?token=secret", ""))
	if !called {
		t.Fatal("public redirect was authenticated")
	}
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if rec.Code == http.StatusMovedPermanently {
		t.Fatal("guard emitted 301")
	}
}

func TestCORSReflectsOnlyConfiguredOrigins(t *testing.T) {
	t.Parallel()

	guard := newGuard(t, ports.LimitConfig{Burst: 10, Refill: time.Second})
	var called bool
	handler := guard(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	preflight := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/links", nil)
	req.Header.Set("Origin", allowedOrigin)
	req.RemoteAddr = "192.0.2.20:1"
	handler.ServeHTTP(preflight, req)
	if preflight.Code != http.StatusNoContent {
		t.Fatalf("preflight = %d, body = %s", preflight.Code, preflight.Body.String())
	}
	if called {
		t.Fatal("preflight reached the handler")
	}
	if got := preflight.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Fatalf("Allow-Origin = %q", got)
	}
	if preflight.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatal("credentials were allowed")
	}

	rejected := httptest.NewRecorder()
	bad := httptest.NewRequest(http.MethodPost, "/api/v1/links", strings.NewReader(`{"url":"https://example.com/docs"}`))
	bad.Header.Set("Origin", "https://evil.example")
	bad.Header.Set("Content-Type", "application/json")
	bad.RemoteAddr = "192.0.2.21:1"
	handler.ServeHTTP(rejected, bad)
	if got := rejected.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("disallowed origin reflected as %q", got)
	}
	if strings.Contains(rejected.Body.String(), "evil.example") {
		t.Fatalf("origin leaked into %s", rejected.Body.String())
	}
}

func TestAdminCrossOriginProtection(t *testing.T) {
	t.Parallel()

	var called bool
	handler := apiHandler(t, func() { called = true })
	req := request(http.MethodDelete, "/api/v1/links/Ab3xyZ9?token=secret", "")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Host = "veylqo.internal"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if called {
		t.Fatal("cross-origin delete reached the handler")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/problem+json") {
		t.Fatalf("Content-Type = %q", got)
	}
	if strings.Contains(rec.Body.String(), "evil.example") || strings.Contains(rec.Body.String(), "token=secret") || strings.Contains(rec.Body.String(), adminToken) {
		t.Fatalf("secret leaked into %s", rec.Body.String())
	}
	if rec.Header().Get("Location") != "" {
		t.Fatal("cross-origin rejection redirected")
	}

	called = false
	trusted := request(http.MethodDelete, "/api/v1/links/Ab3xyZ9", "")
	trusted.Header.Set("Authorization", "Bearer "+adminToken)
	trusted.Header.Set("Origin", allowedOrigin)
	trusted.Header.Set("Sec-Fetch-Site", "cross-site")
	trusted.Host = "veylqo.internal"
	trustedRec := httptest.NewRecorder()
	handler.ServeHTTP(trustedRec, trusted)
	if !called {
		t.Fatal("configured origin was rejected")
	}
	if trustedRec.Code != http.StatusNoContent {
		t.Fatalf("trusted status = %d, body = %s", trustedRec.Code, trustedRec.Body.String())
	}
}

func TestAPIGuardRejectsBadConfig(t *testing.T) {
	t.Parallel()

	limit := ports.LimitConfig{Burst: 1, Refill: time.Second}
	cases := []struct {
		name string
		cfg  ports.APIGuardConfig
		want error
	}{
		{name: "no publishable key", cfg: ports.APIGuardConfig{AdminToken: adminToken, Limit: limit}, want: ports.ErrKeyRequired},
		{name: "no admin token", cfg: ports.APIGuardConfig{PublishableKey: publishableKey, Limit: limit}, want: ports.ErrAdminTokenRequired},
		{
			name: "same credential",
			cfg:  ports.APIGuardConfig{PublishableKey: "shared-secret", AdminToken: "shared-secret", Limit: limit},
			want: ports.ErrKeysNotDistinct,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ports.NewAPIGuard(tt.cfg)
			if err != tt.want {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			if strings.Contains(err.Error(), "shared-secret") || strings.Contains(err.Error(), publishableKey) {
				t.Fatalf("credential leaked into %v", err)
			}
		})
	}

	_, err := ports.NewAPIGuard(ports.APIGuardConfig{
		PublishableKey: publishableKey,
		AdminToken:     adminToken,
		Origins:        []string{"https://app.example/path?token=secret"},
		Limit:          limit,
	})
	if err != ports.ErrInvalidOrigin {
		t.Fatalf("origin error = %v", err)
	}
	if strings.Contains(err.Error(), "token=secret") {
		t.Fatalf("origin leaked into %v", err)
	}
	_, err = ports.NewAPIGuard(ports.APIGuardConfig{
		PublishableKey: publishableKey,
		AdminToken:     adminToken,
		Origins:        []string{"*"},
		Limit:          limit,
	})
	if err != ports.ErrInvalidOrigin {
		t.Fatalf("* origin error = %v", err)
	}
}

func TestAPIGuardRateLimitsBeforeAuth(t *testing.T) {
	t.Parallel()

	guard := newGuard(t, ports.LimitConfig{Burst: 1, Refill: time.Hour})
	handler := guard(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler ran")
	}))
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, request(http.MethodPost, "/api/v1/links", `{"url":"https://example.com/docs"}`))
	assertProblem(t, first, http.StatusUnauthorized, apperrors.Unauthorized)
	second := httptest.NewRecorder()
	req := request(http.MethodPost, "/api/v1/links", `{"url":"https://example.com/docs"}`)
	req.RemoteAddr = firstRemote()
	handler.ServeHTTP(second, req)
	assertProblem(t, second, http.StatusTooManyRequests, apperrors.RateLimited)
	if strings.Contains(second.Body.String(), "192.0.2.1") {
		t.Fatalf("address leaked into %s", second.Body.String())
	}
}

func apiHandler(t *testing.T, called func()) http.Handler {
	t.Helper()
	guard := newGuard(t, ports.LimitConfig{Burst: 20, Refill: time.Second})
	mux := ports.NewMux(app.Application{
		Commands: app.Commands{
			Shorten: shortenFunc(func(context.Context, command.ShortenLink) (command.ShortenLinkResult, error) {
				called()
				code, err := link.NewCode("Ab3xyZ9")
				if err != nil {
					return command.ShortenLinkResult{}, err
				}
				return command.ShortenLinkResult{Code: code}, nil
			}),
			Delete: deleteFunc(func(context.Context, command.DeleteLink) error {
				called()
				return nil
			}),
		},
		Queries: app.Queries{
			Get: getFunc(func(context.Context, query.GetLink) (query.GetLinkResult, error) {
				called()
				code, err := link.NewCode("Ab3xyZ9")
				if err != nil {
					return query.GetLinkResult{}, err
				}
				destination, err := link.NewDestination("https://example.com/docs", link.DestinationConfig{})
				if err != nil {
					return query.GetLinkResult{}, err
				}
				return query.GetLinkResult{Code: code, Destination: destination}, nil
			}),
			Stats: statsFunc(func(context.Context, query.LinkStats) (query.LinkStatsResult, error) {
				called()
				return query.LinkStatsResult{}, nil
			}),
		},
	})
	return guard(mux)
}

func newGuard(t *testing.T, limit ports.LimitConfig) func(http.Handler) http.Handler {
	t.Helper()
	guard, err := ports.NewAPIGuard(ports.APIGuardConfig{
		PublishableKey: publishableKey,
		AdminToken:     adminToken,
		Origins:        []string{allowedOrigin},
		Limit:          limit,
	})
	if err != nil {
		t.Fatal(err)
	}
	return guard
}

func firstRemote() string {
	return httptest.NewRequest(http.MethodGet, "/", nil).RemoteAddr
}
