package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jovrion/veylqo/internal/links/ports"
	"github.com/jovrion/veylqo/internal/links/service"
	"github.com/jovrion/veylqo/internal/migrate"
)

// serve opens the database, optionally migrates, and listens until ctx
// is cancelled. Cancellation is the SIGTERM path: http.Server.Shutdown
// finishes in-flight handlers, then Application.Close drains the click
// batcher. The signal context is already cancelled, so the drain uses a
// fresh timeout. A cancelled drain context would let the process exit
// before the flush (ADR-0009).
//
// Logs do not include the database URL, a destination, a query string,
// a client IP, or a token.
func serve(ctx context.Context, cfg config, logger *slog.Logger) error {
	if ctx == nil {
		return errors.New("serve: nil context")
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	dialect, err := dialectOf(cfg.databaseURL)
	if err != nil {
		return err
	}
	metrics := newCollector(nil)
	wired, err := service.NewApplication(ctx, service.Config{
		DatabaseURL:   cfg.databaseURL,
		AllowedHosts:  cfg.allowedHosts,
		Logger:        logger,
		Metrics:       metrics,
		ClickCapacity: cfg.clickCapacity,
		ClickBatch:    cfg.clickBatch,
		ClickInterval: cfg.clickInterval,
	})
	if err != nil {
		return err
	}
	metrics.setDropped(func() int64 {
		if wired.Batcher == nil {
			return 0
		}
		return wired.Batcher.Dropped()
	})
	closed := false
	closeApp := func(c context.Context) error {
		if closed {
			return nil
		}
		closed = true
		return wired.Close(c)
	}
	fail := func(err error) error {
		drain, cancel := freshTimeout(cfg.shutdownTimeout)
		defer cancel()
		_ = closeApp(drain)
		return err
	}
	if cfg.migrate {
		if err := migrate.Apply(ctx, wired.DB, dialect); err != nil {
			return fail(err)
		}
		logger.Info("migrations applied", "engine", string(dialect))
	} else {
		logger.Info("migrations skipped", "engine", string(dialect))
	}

	ready := &readyState{db: wired.DB}
	handler, err := newHandler(cfg, wired, metrics, ready)
	if err != nil {
		return fail(err)
	}
	ln, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return fail(errors.New("serve: listen failed"))
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		// net/http's own logger can include a client address. Discard
		// that text. A fixed line is enough.
		ErrorLog: log.New(serverErrorWriter{logger: logger}, "", 0),
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(ln)
	}()
	if err := writeAddrFile(cfg.addrFile, ln.Addr().String()); err != nil {
		_ = server.Close()
		<-errCh
		return fail(err)
	}
	logger.Info("listening", "addr", ln.Addr().String())

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fail(errors.New("serve: listener stopped"))
		}
		return fail(nil)
	case <-ctx.Done():
	}

	ready.down.Store(true)
	logger.Info("shutting down")
	drain, cancel := freshTimeout(cfg.shutdownTimeout)
	defer cancel()
	var shutdownErr error
	if err := server.Shutdown(drain); err != nil {
		shutdownErr = errors.New("serve: shutdown")
	}
	if err := closeApp(drain); err != nil && shutdownErr == nil {
		shutdownErr = errors.New("serve: drain")
	}
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) && shutdownErr == nil {
		shutdownErr = errors.New("serve: listener stopped")
	}
	if shutdownErr != nil {
		return shutdownErr
	}
	logger.Info("stopped")
	return nil
}

func freshTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		d = defaultShutdownTimeout
	}
	return context.WithTimeout(context.Background(), d)
}

type serverErrorWriter struct {
	logger *slog.Logger
}

// Write drops the server's text. net/http includes the client address
// in some of those lines. The fixed message has no address.
func (w serverErrorWriter) Write(p []byte) (int, error) {
	if w.logger != nil {
		w.logger.LogAttrs(context.Background(), slog.LevelError, "http server error")
	}
	return len(p), nil
}

func writeAddrFile(path, addr string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "addr-*")
	if err != nil {
		return errors.New("serve: addr file")
	}
	tmp := f.Name()
	if _, err := f.WriteString(addr); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return errors.New("serve: addr file")
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return errors.New("serve: addr file")
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return errors.New("serve: addr file")
	}
	return nil
}

func newHandler(cfg config, wired *service.Application, metrics *collector, ready *readyState) (http.Handler, error) {
	if wired == nil {
		return nil, errors.New("serve: nil application")
	}
	proxies, err := ports.ParseTrustedProxies(cfg.trustedProxies)
	if err != nil {
		return nil, errors.New("config: invalid trusted proxy")
	}
	limit := ports.LimitConfig{
		Burst:          cfg.burst,
		Refill:         cfg.refill,
		TrustedProxies: proxies,
	}
	guard, err := ports.NewAPIGuard(ports.APIGuardConfig{
		PublishableKey: cfg.publishableKey,
		AdminToken:     cfg.adminToken,
		Origins:        cfg.origins,
		Limit:          limit,
	})
	if err != nil {
		return nil, guardError(err)
	}
	redirectLimit, err := ports.NewLimiter(limit)
	if err != nil {
		return nil, errors.New("config: invalid rate limit")
	}
	api := guard(ports.NewMux(wired.Application))
	redirect := redirectLimit.Handler(ports.NewRedirectMux(wired.Application))
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", healthHandler{})
	mux.Handle("GET /readyz", ready)
	mux.Handle("GET /metrics", metrics)
	mux.Handle("/", dispatchHandler{api: api, redirect: redirect})
	return mux, nil
}

func guardError(err error) error {
	switch {
	case errors.Is(err, ports.ErrKeyRequired):
		return errors.New("config: VEYLQO_PUBLISHABLE_KEY is required")
	case errors.Is(err, ports.ErrAdminTokenRequired):
		return errors.New("config: VEYLQO_ADMIN_TOKEN is required")
	case errors.Is(err, ports.ErrKeysNotDistinct):
		return errors.New("config: publishable key and admin token must be distinct")
	case errors.Is(err, ports.ErrInvalidOrigin):
		return errors.New("config: invalid CORS origin")
	case errors.Is(err, ports.ErrInvalidLimit):
		return errors.New("config: invalid rate limit")
	default:
		return errors.New("config: api guard")
	}
}

// dispatchHandler sends API traffic to the guarded mux and every other
// path to the redirect mux. Health, readiness, and metrics are
// registered on the parent, so they never fall through to GET /{code}.
// GET /api is the reserved prefix, not a short code.
type dispatchHandler struct {
	api      http.Handler
	redirect http.Handler
}

func (h dispatchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r != nil && r.URL != nil && strings.HasPrefix(r.URL.Path, "/api/") {
		if h.api == nil {
			http.NotFound(w, r)
			return
		}
		h.api.ServeHTTP(w, r)
		return
	}
	if h.redirect == nil {
		http.NotFound(w, r)
		return
	}
	h.redirect.ServeHTTP(w, r)
}
