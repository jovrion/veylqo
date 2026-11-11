package ports_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/links/ports"
)

func TestParseTrustedProxies(t *testing.T) {
	t.Parallel()

	got, err := ports.ParseTrustedProxies(" 10.0.0.0/8, 192.0.2.9 ,")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].String() != "10.0.0.0/8" || got[1].String() != "192.0.2.9/32" {
		t.Fatalf("proxies = %v", got)
	}
	empty, err := ports.ParseTrustedProxies("  ")
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty = %v, %v", empty, err)
	}
	_, err = ports.ParseTrustedProxies("10.0.0.1?token=secret")
	if err == nil {
		t.Fatal("invalid proxy was accepted")
	}
	if strings.Contains(err.Error(), "token=secret") || strings.Contains(err.Error(), "10.0.0.1") {
		t.Fatalf("proxy leaked into %v", err)
	}
}

func TestLimiterRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := ports.NewLimiter(ports.LimitConfig{})
	if err == nil {
		t.Fatal("zero config was accepted")
	}
}

func TestLimiterTokenBucket(t *testing.T) {
	t.Parallel()

	clock := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	limiter := newLimiter(t, ports.LimitConfig{
		Burst:  1,
		Refill: time.Second,
		Now: func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			return clock
		},
	})
	handler := limiter.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, from("192.0.2.10:1000"))
	if first.Code != http.StatusNoContent {
		t.Fatalf("first status = %d", first.Code)
	}

	denied := httptest.NewRecorder()
	req := from("192.0.2.10:1000")
	req.Header.Set("X-Forwarded-For", "203.0.113.44")
	handler.ServeHTTP(denied, req)
	assertProblem(t, denied, http.StatusTooManyRequests, apperrors.RateLimited)
	if strings.Contains(denied.Body.String(), "192.0.2.10") || strings.Contains(denied.Body.String(), "203.0.113.44") {
		t.Fatalf("address leaked into %s", denied.Body.String())
	}
	if denied.Header().Get("Location") != "" || denied.Code == http.StatusMovedPermanently {
		t.Fatal("rate limit must not redirect")
	}

	other := httptest.NewRecorder()
	handler.ServeHTTP(other, from("192.0.2.11:1000"))
	if other.Code != http.StatusNoContent {
		t.Fatalf("other client status = %d, want separate bucket", other.Code)
	}

	mu.Lock()
	clock = clock.Add(time.Second)
	mu.Unlock()
	refilled := httptest.NewRecorder()
	handler.ServeHTTP(refilled, from("192.0.2.10:1000"))
	if refilled.Code != http.StatusNoContent {
		t.Fatalf("refilled status = %d", refilled.Code)
	}
}

func TestLimiterIgnoresUntrustedForwardedHeader(t *testing.T) {
	t.Parallel()

	limiter := newLimiter(t, ports.LimitConfig{Burst: 1, Refill: time.Hour})
	handler := limiter.Handler(statusHandler(http.StatusNoContent))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, forwarded("198.51.100.8:9", "203.0.113.1"))
	if first.Code != http.StatusNoContent {
		t.Fatalf("first = %d", first.Code)
	}
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, forwarded("198.51.100.8:9", "203.0.113.2"))
	assertProblem(t, second, http.StatusTooManyRequests, apperrors.RateLimited)
}

func TestLimiterUsesForwardedHeaderOnlyForTrustedPeer(t *testing.T) {
	t.Parallel()

	proxies, err := ports.ParseTrustedProxies("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	limiter := newLimiter(t, ports.LimitConfig{
		Burst:          1,
		Refill:         time.Hour,
		TrustedProxies: proxies,
	})
	handler := limiter.Handler(statusHandler(http.StatusNoContent))

	one := httptest.NewRecorder()
	handler.ServeHTTP(one, forwarded("10.1.2.3:443", "203.0.113.50"))
	if one.Code != http.StatusNoContent {
		t.Fatalf("first client = %d", one.Code)
	}
	two := httptest.NewRecorder()
	handler.ServeHTTP(two, forwarded("10.1.2.3:443", "203.0.113.51"))
	if two.Code != http.StatusNoContent {
		t.Fatalf("second client = %d, trusted proxy collapsed buckets", two.Code)
	}
	again := httptest.NewRecorder()
	handler.ServeHTTP(again, forwarded("10.1.2.3:443", "203.0.113.50, 10.0.0.4"))
	assertProblem(t, again, http.StatusTooManyRequests, apperrors.RateLimited)
	if strings.Contains(again.Body.String(), "203.0.113.50") || strings.Contains(again.Body.String(), "10.1.2.3") {
		t.Fatalf("address leaked into %s", again.Body.String())
	}
}

func TestLimiterIgnoresUnparseableForwardedHeader(t *testing.T) {
	t.Parallel()

	proxies, err := ports.ParseTrustedProxies("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	limiter := newLimiter(t, ports.LimitConfig{
		Burst:          1,
		Refill:         time.Hour,
		TrustedProxies: proxies,
	})
	handler := limiter.Handler(statusHandler(http.StatusNoContent))
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, forwarded("10.9.9.9:1", "token=secret"))
	if first.Code != http.StatusNoContent {
		t.Fatalf("first = %d", first.Code)
	}
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, forwarded("10.9.9.9:1", "also-not-an-ip"))
	assertProblem(t, second, http.StatusTooManyRequests, apperrors.RateLimited)
	if strings.Contains(second.Body.String(), "token=secret") {
		t.Fatalf("forwarded header leaked into %s", second.Body.String())
	}
}

func TestLimiterFailClosedOnUnparseablePeer(t *testing.T) {
	t.Parallel()

	limiter := newLimiter(t, ports.LimitConfig{Burst: 5, Refill: time.Second})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/Ab3xyZ9?token=secret", nil)
	req.RemoteAddr = "not-a-host"
	limiter.Handler(statusHandler(http.StatusNoContent)).ServeHTTP(rec, req)
	assertProblem(t, rec, http.StatusTooManyRequests, apperrors.RateLimited)
	if strings.Contains(rec.Body.String(), "not-a-host") || strings.Contains(rec.Body.String(), "token=secret") {
		t.Fatalf("peer leaked into %s", rec.Body.String())
	}
	if rec.Code == http.StatusMovedPermanently {
		t.Fatal("limiter redirected")
	}
}

func TestLimiterIPv6Peer(t *testing.T) {
	t.Parallel()

	limiter := newLimiter(t, ports.LimitConfig{Burst: 1, Refill: time.Hour})
	handler := limiter.Handler(statusHandler(http.StatusNoContent))
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, from("[2001:db8::5]:443"))
	if first.Code != http.StatusNoContent {
		t.Fatalf("first = %d", first.Code)
	}
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, from("[2001:db8::5]:443"))
	assertProblem(t, second, http.StatusTooManyRequests, apperrors.RateLimited)
	if strings.Contains(second.Body.String(), "2001:db8") {
		t.Fatalf("address leaked into %s", second.Body.String())
	}
}

func TestLimiterIsSafeForConcurrentClients(t *testing.T) {
	t.Parallel()

	const burst = 8
	limiter := newLimiter(t, ports.LimitConfig{Burst: burst, Refill: time.Hour})
	var allowed atomic.Int32
	handler := limiter.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		allowed.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	var group sync.WaitGroup
	for range 32 {
		group.Go(func() {
			handler.ServeHTTP(httptest.NewRecorder(), from("192.0.2.40:1"))
		})
	}
	group.Wait()
	if got := allowed.Load(); got != burst {
		t.Fatalf("allowed = %d, want %d", got, burst)
	}
}

func newLimiter(t *testing.T, cfg ports.LimitConfig) *ports.Limiter {
	t.Helper()
	limiter, err := ports.NewLimiter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return limiter
}

func from(remote string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/Ab3xyZ9", nil)
	req.RemoteAddr = remote
	return req
}

func forwarded(remote, xff string) *http.Request {
	req := from(remote)
	req.Header.Set("X-Forwarded-For", xff)
	return req
}

func statusHandler(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	})
}
