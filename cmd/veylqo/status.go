package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jovrion/veylqo/internal/common/decorator"
	"github.com/jovrion/veylqo/internal/common/httpx"
)

type healthBody struct {
	Status string `json:"status"`
}

type healthHandler struct{}

func (healthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	writeHealth(w, r, http.StatusOK, "ok")
}

// readyState is readiness. down is set before Server.Shutdown so a
// probe during drain is 503. A database ping failure is also 503.
// The body is the spec's Health object, not problem+json, and it does
// not include the ping error (that text can repeat a DSN).
type readyState struct {
	db   *sql.DB
	down atomic.Bool
}

func (s *readyState) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.down.Load() || s.db == nil {
		writeHealth(w, r, http.StatusServiceUnavailable, "not-ready")
		return
	}
	ctx := r.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ping, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := s.db.PingContext(ping); err != nil {
		writeHealth(w, r, http.StatusServiceUnavailable, "not-ready")
		return
	}
	writeHealth(w, r, http.StatusOK, "ready")
}

func writeHealth(w http.ResponseWriter, r *http.Request, status int, state string) {
	if err := httpx.Encode(w, status, healthBody{Status: state}); err != nil {
		httpx.WriteProblem(w, r, err)
	}
}

// collector counts handler calls and dropped clicks. Labels are the
// handler name and the error class. A URL, a raw IP, or a token is not
// a label (ADR-0009). The /metrics route is this type, not the
// decorator.Metrics interface itself.
type collector struct {
	mu      sync.Mutex
	calls   map[string]int64
	dropped func() int64
}

func newCollector(dropped func() int64) *collector {
	return &collector{
		calls:   make(map[string]int64),
		dropped: dropped,
	}
}

func (c *collector) setDropped(fn func() int64) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.dropped = fn
	c.mu.Unlock()
}

// Observe implements decorator.Metrics.
func (c *collector) Observe(observation decorator.Observation) {
	if c == nil {
		return
	}
	name := metricLabel(observation.Name)
	class := metricLabel(observation.Class)
	if class == "" {
		class = "ok"
	}
	key := name + "\x00" + class
	c.mu.Lock()
	c.calls[key]++
	c.mu.Unlock()
}

func (c *collector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if w == nil {
		return
	}
	body := c.render()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=0")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (c *collector) render() []byte {
	if c == nil {
		return []byte("veylqo_clicks_dropped 0\n")
	}
	c.mu.Lock()
	keys := make([]string, 0, len(c.calls))
	for key := range c.calls {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var b strings.Builder
	b.WriteString("# counts only; no URL, IP, or token\n")
	for _, key := range keys {
		name, class, _ := strings.Cut(key, "\x00")
		fmt.Fprintf(&b, "veylqo_handler_calls{name=%q,class=%q} %d\n", name, class, c.calls[key])
	}
	droppedFn := c.dropped
	c.mu.Unlock()
	dropped := int64(0)
	if droppedFn != nil {
		dropped = droppedFn()
	}
	fmt.Fprintf(&b, "veylqo_clicks_dropped %d\n", dropped)
	return []byte(b.String())
}

func metricLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") || strings.Contains(value, "?") || strings.ContainsAny(value, " \t\r\n\"{}") {
		return "redacted"
	}
	for _, r := range value {
		allowed := r == '_' || r == '-' ||
			(r >= '0' && r <= '9') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= 'a' && r <= 'z')
		if r > 127 || !allowed {
			return "redacted"
		}
	}
	return value
}
