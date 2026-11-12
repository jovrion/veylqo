package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	json "encoding/json/v2"

	"github.com/jovrion/veylqo/internal/common/decorator"
)

const (
	testPublishable = "pub-key-not-logged"
	testAdmin       = "admin-token-not-logged"
	testSecretQuery = "secret-query"
	testDestination = "https://example.com/landing?token=" + testSecretQuery
)

func TestConfigRequiresSecretsAndAllowlist(t *testing.T) {
	t.Parallel()

	base := map[string]string{
		"VEYLQO_DB_URL":          "sqlite://file:veylqo.db",
		"VEYLQO_ALLOWED_HOSTS":   "example.com",
		"VEYLQO_PUBLISHABLE_KEY": testPublishable,
		"VEYLQO_ADMIN_TOKEN":     testAdmin,
	}
	for _, key := range []string{"VEYLQO_DB_URL", "VEYLQO_ALLOWED_HOSTS", "VEYLQO_PUBLISHABLE_KEY", "VEYLQO_ADMIN_TOKEN"} {
		env := cloneMap(base)
		delete(env, key)
		_, err := parseConfig(nil, lookup(env))
		if err == nil {
			t.Fatalf("missing %s was accepted", key)
		}
		if strings.Contains(err.Error(), testPublishable) || strings.Contains(err.Error(), testAdmin) {
			t.Fatalf("error leaked a credential: %v", err)
		}
	}
}

func TestConfigRejectsIdenticalKeysWithoutEchoingThem(t *testing.T) {
	t.Parallel()

	const shared = "same-secret-value"
	_, err := parseConfig(nil, lookup(map[string]string{
		"VEYLQO_DB_URL":          "sqlite://file:veylqo.db",
		"VEYLQO_ALLOWED_HOSTS":   "",
		"VEYLQO_PUBLISHABLE_KEY": shared,
		"VEYLQO_ADMIN_TOKEN":     shared,
	}))
	if err == nil {
		t.Fatal("identical keys were accepted")
	}
	if strings.Contains(err.Error(), shared) {
		t.Fatalf("error echoed the key: %v", err)
	}
}

func TestConfigFlagOverridesAddrAndMigrate(t *testing.T) {
	t.Parallel()

	cfg, err := parseConfig([]string{"-addr", "127.0.0.1:9", "-migrate=false"}, lookup(map[string]string{
		"VEYLQO_DB_URL":          "postgres://veylqo:secret-password@192.0.2.10/veylqo",
		"VEYLQO_ALLOWED_HOSTS":   "example.com",
		"VEYLQO_PUBLISHABLE_KEY": testPublishable,
		"VEYLQO_ADMIN_TOKEN":     testAdmin,
		"VEYLQO_ADDR":            ":8080",
		"VEYLQO_MIGRATE":         "true",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.addr != "127.0.0.1:9" {
		t.Fatalf("addr = %q", cfg.addr)
	}
	if cfg.migrate {
		t.Fatal("flag did not disable migrations")
	}
	if !strings.Contains(strings.Join(cfg.secrets(), " "), "secret-password") {
		t.Fatal("password was not treated as a secret")
	}
	if got := redact("open failed secret-password", cfg.secrets()); got != "redacted" {
		t.Fatalf("redact = %q", got)
	}
}

func TestConfigHelp(t *testing.T) {
	t.Parallel()

	_, err := parseConfig([]string{"-h"}, lookup(map[string]string{}))
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help error = %v", err)
	}
}

func TestReadyzNotReadyDoesNotLeakDSN(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "veylqo.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	ready := &readyState{db: db}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz?token="+testSecretQuery, nil)
	ready.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content type = %q", rec.Header().Get("Content-Type"))
	}
	assertHealth(t, rec.Body.Bytes(), "not-ready")
	if strings.Contains(rec.Body.String(), path) || strings.Contains(rec.Body.String(), testSecretQuery) {
		t.Fatalf("body leaked path or token: %s", rec.Body.String())
	}

	down := &readyState{db: db}
	down.down.Store(true)
	downRec := httptest.NewRecorder()
	down.ServeHTTP(downRec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if downRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("down status = %d", downRec.Code)
	}
	assertHealth(t, downRec.Body.Bytes(), "not-ready")
}

func TestMetricsOmitURLIPAndToken(t *testing.T) {
	t.Parallel()

	c := newCollector(func() int64 { return 2 })
	c.Observe(decorator.Observation{Name: "RecordClick"})
	c.Observe(decorator.Observation{
		Name:  "https://example.com/landing?token=" + testSecretQuery,
		Class: "10.0.0.8",
	})
	body := string(c.render())
	if !strings.Contains(body, `name="RecordClick"`) || !strings.Contains(body, `class="ok"`) {
		t.Fatalf("body = %s", body)
	}
	if !strings.Contains(body, "veylqo_clicks_dropped 2") {
		t.Fatalf("body = %s", body)
	}
	if strings.Contains(body, testSecretQuery) || strings.Contains(body, "example.com") || strings.Contains(body, "10.0.0.8") {
		t.Fatalf("metrics leaked a label: %s", body)
	}
}

func TestHealthAndMigrateFlag(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "veylqo.db")
	var logs bytes.Buffer
	cfg := runningConfig(t, dbPath, false)
	base, stop := startServe(t, cfg, slog.New(slog.NewTextHandler(&logs, nil)))
	defer stop()

	client := &http.Client{Timeout: 2 * time.Second}
	health := get(t, client, base+"/healthz")
	if health.StatusCode != http.StatusOK {
		t.Fatalf("healthz = %d", health.StatusCode)
	}
	if health.Header.Get("Cache-Control") != "private, max-age=0" {
		t.Fatalf("cache-control = %q", health.Header.Get("Cache-Control"))
	}
	body, err := io.ReadAll(health.Body)
	health.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	assertHealth(t, body, "ok")

	ready := get(t, client, base+"/readyz")
	if ready.StatusCode != http.StatusOK {
		t.Fatalf("readyz = %d", ready.StatusCode)
	}
	readyBody, err := io.ReadAll(ready.Body)
	ready.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	assertHealth(t, readyBody, "ready")

	if _, err := clickTotal(dbPath); err == nil {
		t.Fatal("migrations ran despite VEYLQO_MIGRATE=false")
	} else if !strings.Contains(err.Error(), "no such table") {
		t.Fatalf("schema probe = %v", err)
	}
	logged := logs.String()
	for _, secret := range []string{testPublishable, testAdmin, dbPath, testSecretQuery} {
		if strings.Contains(logged, secret) {
			t.Fatalf("log leaked %q: %s", secret, logged)
		}
	}
}

func TestSIGTERMDoesNotSkipBatcherFlush(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "veylqo")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = packageDir(t)
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "veylqo.db")
	addrFile := filepath.Join(dir, "addr")
	cmd := exec.Command(bin)
	cmd.Env = []string{
		"VEYLQO_DB_URL=sqlite://file:" + filepath.ToSlash(dbPath) + "?mode=rwc",
		"VEYLQO_ADDR=127.0.0.1:0",
		"VEYLQO_ADDR_FILE=" + addrFile,
		"VEYLQO_ALLOWED_HOSTS=example.com",
		"VEYLQO_PUBLISHABLE_KEY=" + testPublishable,
		"VEYLQO_ADMIN_TOKEN=" + testAdmin,
		"VEYLQO_CLICK_INTERVAL=1h",
		"VEYLQO_CLICK_BATCH=1000",
		"VEYLQO_SHUTDOWN_TIMEOUT=10s",
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	base := "http://" + waitAddrFile(t, addrFile, waitDone)
	client := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	health := get(t, client, base+"/healthz")
	if health.StatusCode != http.StatusOK || health.Header.Get("Cache-Control") != "private, max-age=0" {
		t.Fatalf("healthz = %d cache = %q", health.StatusCode, health.Header.Get("Cache-Control"))
	}
	health.Body.Close()
	ready := get(t, client, base+"/readyz")
	if ready.StatusCode != http.StatusOK {
		t.Fatalf("readyz = %d", ready.StatusCode)
	}
	ready.Body.Close()

	code := shorten(t, client, base)
	redirect := get(t, client, base+"/"+code)
	if redirect.StatusCode != http.StatusFound {
		t.Fatalf("redirect = %d, want 302", redirect.StatusCode)
	}
	redirect.Body.Close()

	deadline := time.Now().Add(5 * time.Second)
	var metrics string
	for time.Now().Before(deadline) {
		res := get(t, client, base+"/metrics")
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		metrics = string(body)
		if strings.Contains(metrics, `name="RecordClick"`) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(metrics, `name="RecordClick"`) {
		t.Fatalf("click was not queued before SIGTERM: %s", metrics)
	}
	if strings.Contains(metrics, testSecretQuery) || strings.Contains(metrics, "example.com") {
		t.Fatalf("metrics leaked a destination: %s", metrics)
	}
	before, err := clickTotal(dbPath)
	if err != nil {
		t.Fatalf("count before signal: %v", err)
	}
	if before != 0 {
		t.Fatalf("click was flushed before SIGTERM: %d", before)
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-waitDone:
		if err != nil {
			t.Fatalf("process exit after SIGTERM: %v\n%s", err, stderr.String())
		}
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("SIGTERM did not finish shutdown")
	}
	after, err := clickTotal(dbPath)
	if err != nil {
		t.Fatalf("count after signal: %v", err)
	}
	if after != 1 {
		t.Fatalf("flushed clicks = %d, want 1; stderr = %s", after, stderr.String())
	}
	logged := stderr.String()
	for _, secret := range []string{testSecretQuery, testPublishable, testAdmin, testDestination, "token="} {
		if strings.Contains(logged, secret) {
			t.Fatalf("stderr leaked %q", secret)
		}
	}
}

func runningConfig(t *testing.T, dbPath string, migrate bool) config {
	t.Helper()
	env := map[string]string{
		"VEYLQO_DB_URL":           "sqlite://file:" + filepath.ToSlash(dbPath) + "?mode=rwc",
		"VEYLQO_ALLOWED_HOSTS":    "example.com",
		"VEYLQO_PUBLISHABLE_KEY":  testPublishable,
		"VEYLQO_ADMIN_TOKEN":      testAdmin,
		"VEYLQO_CLICK_INTERVAL":   "1h",
		"VEYLQO_CLICK_BATCH":      "1000",
		"VEYLQO_SHUTDOWN_TIMEOUT": "5s",
		"VEYLQO_ADDR_FILE":        filepath.Join(filepath.Dir(dbPath), "addr"),
	}
	if !migrate {
		env["VEYLQO_MIGRATE"] = "false"
	}
	cfg, err := parseConfig(nil, lookup(env))
	if err != nil {
		t.Fatal(err)
	}
	cfg.addr = "127.0.0.1:0"
	return cfg
}

func startServe(t *testing.T, cfg config, logger *slog.Logger) (string, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- serve(ctx, cfg, logger)
	}()
	addr := waitAddrFile(t, cfg.addrFile, errCh)
	return "http://" + addr, func() {
		t.Helper()
		cancel()
		select {
		case err := <-errCh:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(8 * time.Second):
			t.Error("shutdown timed out")
		}
	}
}

func shorten(t *testing.T, client *http.Client, base string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/links", strings.NewReader(`{"url":"`+testDestination+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+testPublishable)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("shorten = %d, body = %s", res.StatusCode, redact(string(body), []string{testSecretQuery, testPublishable, testAdmin}))
	}
	var decoded struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Code == "" {
		t.Fatal("empty code")
	}
	return decoded.Code
}

func get(t *testing.T, client *http.Client, rawURL string) *http.Response {
	t.Helper()
	res, err := client.Get(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func clickTotal(path string) (int64, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro&_busy_timeout=1000")
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var n int64
	err = db.QueryRow(`SELECT COALESCE(SUM(click_count), 0) FROM clicks`).Scan(&n)
	return n, err
}

func waitAddrFile(t *testing.T, path string, errCh <-chan error) string {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-errCh:
			t.Fatalf("process exited before listen: %v", err)
		default:
		}
		body, err := os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(body)) != "" {
			return strings.TrimSpace(string(body))
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for listen address")
	return ""
}

func assertHealth(t *testing.T, body []byte, want string) {
	t.Helper()
	var decoded healthBody
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("health body: %v (%s)", err, body)
	}
	if decoded.Status != want {
		t.Fatalf("status = %q, want %q", decoded.Status, want)
	}
}

func packageDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

func lookup(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	}
}

func cloneMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
