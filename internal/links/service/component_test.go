package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	json "encoding/json/v2"

	"github.com/jovrion/veylqo/internal/links/ports"
)

const (
	componentPublishableKey = "component-publishable-key"
	componentAdminToken     = "component-admin-token"
)

// TestComponentShortenResolveStats drives the composed HTTP edge:
// shorten, resolve 302, then stats. The server is httptest.NewTestServer
// without Start, so the client dials an in-memory listener and never
// opens a socket. The redirect is not followed, so the destination is
// not fetched.
func TestComponentShortenResolveStats(t *testing.T) {
	wired, err := NewComponentTestApplication(Config{
		AllowedHosts:  []string{"example.com"},
		ClickBatch:    1,
		ClickInterval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := wired.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if wired.DB != nil {
		t.Fatal("component application opened a database")
	}

	var seen []string
	handler := componentHandler(t, wired, func(r *http.Request) {
		if r == nil || r.URL == nil {
			t.Error("nil request")
			return
		}
		seen = append(seen, r.Method+" "+r.URL.Path)
		if r.URL.Host == "example.com" || strings.Contains(r.URL.RawQuery, "utm_") {
			t.Errorf("destination was requested: %s %s", r.Method, r.URL.Path)
		}
	})
	server := httptest.NewTestServer(t, handler)
	client := server.Client()
	if server.Listener != nil {
		t.Fatal("component test bound a real listener")
	}
	client.Timeout = 5 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	const rawURL = "https://example.com/docs?utm_source=ad"
	const storedURL = "https://example.com/docs"
	createBody := `{"url":"` + rawURL + `"}`
	createReq, err := http.NewRequest(http.MethodPost, "http://veylqo.test/api/v1/links", strings.NewReader(createBody))
	if err != nil {
		t.Fatal(err)
	}
	createReq.Header.Set("Authorization", "Bearer "+componentPublishableKey)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := client.Do(createReq)
	if err != nil {
		t.Fatal(err)
	}
	createPayload := readBody(t, createResp)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("shorten status = %d, body = %s", createResp.StatusCode, createPayload)
	}
	if strings.Contains(createPayload, "utm_source") || strings.Contains(createPayload, "example.com") {
		t.Fatalf("shorten body leaked the destination: %s", createPayload)
	}
	var created struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(createPayload), &created); err != nil {
		t.Fatal(err)
	}
	if created.Code == "" || strings.Contains(created.Code, "-") {
		t.Fatalf("public code = %q, want a base62 code", created.Code)
	}

	resolveReq, err := http.NewRequest(http.MethodGet, "http://veylqo.test/"+created.Code, nil)
	if err != nil {
		t.Fatal(err)
	}
	resolveResp, err := client.Do(resolveReq)
	if err != nil {
		t.Fatal(err)
	}
	resolvePayload := readBody(t, resolveResp)
	if resolveResp.StatusCode != http.StatusFound {
		t.Fatalf("resolve status = %d, want 302, body = %s", resolveResp.StatusCode, resolvePayload)
	}
	if resolveResp.StatusCode == http.StatusMovedPermanently {
		t.Fatal("resolve status = 301")
	}
	if got := resolveResp.Header.Get("Location"); got != storedURL {
		t.Fatalf("Location = %q, want stored destination without the tracking query", got)
	}
	if got := resolveResp.Header.Get("Cache-Control"); got != "private, max-age=0" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if resolvePayload != "" {
		t.Fatalf("resolve body = %s", resolvePayload)
	}

	statsURL := "http://veylqo.test/api/v1/links/" + created.Code + "/stats"
	deadline := time.Now().Add(2 * time.Second)
	var clicks int64
	for {
		clicks = componentStats(t, client, statsURL)
		if clicks == 1 || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if clicks != 1 {
		t.Fatalf("stats clicks = %d, want 1 after the redirect was recorded", clicks)
	}
	for _, path := range seen {
		if strings.Contains(path, "/docs") {
			t.Fatalf("destination path was requested: %s", path)
		}
	}
}

func componentHandler(t *testing.T, wired *Application, observe func(*http.Request)) http.Handler {
	t.Helper()
	limit := ports.LimitConfig{Burst: 64, Refill: time.Millisecond}
	guard, err := ports.NewAPIGuard(ports.APIGuardConfig{
		PublishableKey: componentPublishableKey,
		AdminToken:     componentAdminToken,
		Limit:          limit,
	})
	if err != nil {
		t.Fatal(err)
	}
	redirectLimit, err := ports.NewLimiter(limit)
	if err != nil {
		t.Fatal(err)
	}
	api := guard(ports.NewMux(wired.Application))
	redirect := redirectLimit.Handler(ports.NewRedirectMux(wired.Application))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if observe != nil {
			observe(r)
		}
		if r != nil && r.URL != nil && strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		redirect.ServeHTTP(w, r)
	})
}

func componentStats(t *testing.T, client *http.Client, url string) int64 {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+componentAdminToken)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	payload := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stats status = %d, body = %s", resp.StatusCode, payload)
	}
	if strings.Contains(payload, "example.com") || strings.Contains(payload, "utm_") {
		t.Fatalf("stats body leaked the destination: %s", payload)
	}
	var stats struct {
		Clicks int64 `json:"clicks"`
	}
	if err := json.Unmarshal([]byte(payload), &stats); err != nil {
		t.Fatal(err)
	}
	return stats.Clicks
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	if resp == nil || resp.Body == nil {
		t.Fatal("nil response")
	}
	body, err := io.ReadAll(resp.Body)
	if cerr := resp.Body.Close(); cerr != nil && err == nil {
		err = cerr
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
