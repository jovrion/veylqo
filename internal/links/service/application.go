// Package service wires adapters into the link application. It holds no
// use-case logic (ADR-0002). Drivers are not imported here. cmd/veylqo
// blank-imports them before NewApplication, and tests may register a
// driver in *_test.go (ADR-0003).
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/jovrion/veylqo/internal/common/decorator"
	"github.com/jovrion/veylqo/internal/links/adapters"
	"github.com/jovrion/veylqo/internal/links/app"
	"github.com/jovrion/veylqo/internal/links/app/command"
	"github.com/jovrion/veylqo/internal/links/domain/link"
)

const (
	sqliteDriver   = "sqlite"
	postgresDriver = "pgx"

	defaultCacheCapacity = 1024
	defaultCacheTTL      = 30 * time.Second
	defaultClickCapacity = 1024
	defaultClickBatch    = 64
	defaultClickInterval = time.Second

	sqliteBusyTimeout = "5000"
)

// openSQL opens a database/sql pool. Tests replace it to record the
// driver name. Production code does not import a driver; the name is
// "sqlite" or "pgx", which cmd/veylqo has already registered.
var openSQL = sql.Open

// Config is process configuration. DatabaseURL is VEYLQO_DB_URL. It is
// postgres:// or sqlite://. AllowedHosts is VEYLQO_ALLOWED_HOSTS. An
// empty list is a closed allowlist, not an error. Zero cache and batcher
// fields use the defaults. This struct is not logged: DatabaseURL may
// carry a password.
type Config struct {
	DatabaseURL   string
	AllowedHosts  []string
	Logger        *slog.Logger
	Metrics       decorator.Metrics
	Now           func() time.Time
	CacheCapacity int
	CacheTTL      time.Duration
	ClickCapacity int
	ClickBatch    int
	ClickInterval time.Duration
}

// Application is the assembled process graph. Commands and queries are
// the embedded app.Application. This type does not listen, migrate, or
// decide allowlists, expiry, or cleaning.
type Application struct {
	app.Application

	// DB is the pool for sqlite or postgres. It is nil when the engine is
	// memory. cmd runs migrations on it. Close closes it.
	DB *sql.DB

	// Store is the engine adapter before the resolve cache. Use-cases see
	// the cache, so Resolve and Delete share one entry.
	Store link.Repository

	// Clicks is the count store. The batcher writes it. Stats reads it.
	Clicks link.ClickRepository

	// Batcher accepts clicks without waiting. Close drains it. A click
	// can be lost if the process dies before flush (ADR-0009).
	Batcher *adapters.ClickBatcher
}

// NewApplication selects the adapter from cfg.DatabaseURL and wires it
// into app. sqlite:// uses the sqlite adapter and does not open pgx.
// postgres:// uses the postgres adapter. Memory is not selected here.
// Migrations are not applied; cmd does that before serving (ADR-0005).
// A nil ctx, an empty URL, or an unsupported scheme is an error. The
// error text does not include the URL, a password, or a host.
func NewApplication(ctx context.Context, cfg Config) (*Application, error) {
	if ctx == nil {
		return nil, errors.New("application: nil context")
	}
	target, err := parseDatabaseURL(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	db, err := openSQL(target.driver, target.dsn)
	if err != nil {
		return nil, hideDatabaseURL(cfg.DatabaseURL, err)
	}
	if target.kind == "sqlite" {
		db.SetMaxOpenConns(1)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, hideDatabaseURL(cfg.DatabaseURL, err)
	}
	var (
		store  link.Repository
		clicks link.ClickRepository
	)
	switch target.kind {
	case "sqlite":
		store = adapters.NewSQLiteRepository(db)
		clicks = adapters.NewSQLiteClicks(db)
	case "postgres":
		store = adapters.NewPostgresRepository(db)
		clicks = adapters.NewPostgresClicks(db)
	default:
		_ = db.Close()
		return nil, errors.New("database url: unsupported scheme")
	}
	wired, err := assemble(cfg, store, clicks, db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return wired, nil
}

// NewComponentTestApplication wires the memory adapter. It does not read
// DatabaseURL and it does not open a database. Close drains the batcher.
func NewComponentTestApplication(cfg Config) (*Application, error) {
	store := adapters.NewMemoryRepository()
	return assemble(cfg, store, &memoryClicks{}, nil)
}

func assemble(cfg Config, store link.Repository, clicks link.ClickRepository, db *sql.DB) (*Application, error) {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	capacity, err := positiveOrDefault(cfg.CacheCapacity, defaultCacheCapacity, "cache capacity")
	if err != nil {
		return nil, err
	}
	ttl := cfg.CacheTTL
	if ttl == 0 {
		ttl = defaultCacheTTL
	}
	if ttl < 0 {
		return nil, errors.New("cache ttl must be positive")
	}
	cache, err := adapters.NewResolveCache(store, now, adapters.ResolveCacheConfig{
		Capacity: capacity,
		TTL:      ttl,
	})
	if err != nil {
		return nil, err
	}
	allow, err := command.NewAllowlist(cfg.AllowedHosts)
	if err != nil {
		return nil, err
	}
	clickCapacity, err := positiveOrDefault(cfg.ClickCapacity, defaultClickCapacity, "click capacity")
	if err != nil {
		return nil, err
	}
	clickBatch, err := positiveOrDefault(cfg.ClickBatch, defaultClickBatch, "click batch")
	if err != nil {
		return nil, err
	}
	interval := cfg.ClickInterval
	if interval == 0 {
		interval = defaultClickInterval
	}
	if interval < 0 {
		return nil, errors.New("click interval must be positive")
	}
	batcher, err := adapters.NewClickBatcher(clicks, adapters.ClickBatcherConfig{
		Capacity: clickCapacity,
		Batch:    clickBatch,
		Interval: interval,
	})
	if err != nil {
		return nil, err
	}
	application := app.NewApplication(app.Dependencies{
		Links:    cache,
		Clicks:   clicks,
		Recorder: batcher,
		Codes:    adapters.NewCodeGenerator(),
		Cleaner:  link.DefaultCleaner(),
		Allow:    allow,
		Now:      now,
		Logger:   cfg.Logger,
		Metrics:  cfg.Metrics,
	})
	return &Application{
		Application: application,
		DB:          db,
		Store:       store,
		Clicks:      clicks,
		Batcher:     batcher,
	}, nil
}

// Close drains queued clicks and closes the pool. A nil context is an
// error. A second call shuts the batcher down again and closes the pool
// again; cmd should call it once. The error text has no URL.
func (a *Application) Close(ctx context.Context) error {
	if a == nil {
		return errors.New("application: nil")
	}
	if ctx == nil {
		return errors.New("application: nil context")
	}
	var err error
	if a.Batcher != nil {
		err = a.Batcher.Shutdown(ctx)
	}
	if a.DB != nil {
		if cerr := a.DB.Close(); err == nil {
			err = cerr
		}
	}
	return err
}

type databaseTarget struct {
	kind   string
	driver string
	dsn    string
}

func parseDatabaseURL(raw string) (databaseTarget, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return databaseTarget{}, errors.New("database url: empty")
	}
	scheme, rest, ok := strings.Cut(raw, "://")
	if !ok || scheme == "" || strings.ContainsAny(scheme, "/?#@") {
		return databaseTarget{}, errors.New("database url: scheme required")
	}
	switch strings.ToLower(scheme) {
	case "sqlite":
		dsn, err := sqliteDSN(rest)
		if err != nil {
			return databaseTarget{}, err
		}
		return databaseTarget{kind: "sqlite", driver: sqliteDriver, dsn: dsn}, nil
	case "postgres":
		if strings.TrimSpace(rest) == "" {
			return databaseTarget{}, errors.New("database url: empty postgres target")
		}
		return databaseTarget{kind: "postgres", driver: postgresDriver, dsn: "postgres://" + rest}, nil
	default:
		return databaseTarget{}, errors.New("database url: unsupported scheme")
	}
}

func sqliteDSN(rest string) (string, error) {
	rest = strings.TrimSpace(rest)
	if rest == "" || strings.HasPrefix(rest, "?") {
		return "", errors.New("database url: empty sqlite path")
	}
	if !strings.Contains(rest, "_busy_timeout=") {
		rest = appendQuery(rest, "_busy_timeout="+sqliteBusyTimeout)
	}
	if !strings.Contains(rest, "_txlock=") {
		rest = appendQuery(rest, "_txlock=immediate")
	}
	return rest, nil
}

func appendQuery(dsn, pair string) string {
	if strings.Contains(dsn, "?") {
		return dsn + "&" + pair
	}
	return dsn + "?" + pair
}

func positiveOrDefault(v, fallback int, name string) (int, error) {
	if v == 0 {
		return fallback, nil
	}
	if v < 0 {
		return 0, fmt.Errorf("%s must be positive", name)
	}
	return v, nil
}

// hideDatabaseURL drops a driver error that repeats the URL, a password,
// a username, or an IP host. Callers must not log the returned error as
// if it were a destination. Our own text never includes the URL.
func hideDatabaseURL(raw string, err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if raw != "" && strings.Contains(msg, raw) {
		return errors.New("database: open failed")
	}
	parsed, parseErr := url.Parse(raw)
	if parseErr != nil || parsed == nil {
		return fmt.Errorf("database: open: %w", err)
	}
	if user := parsed.User; user != nil {
		if name := user.Username(); name != "" && strings.Contains(msg, name) {
			return errors.New("database: open failed")
		}
		if pw, ok := user.Password(); ok && pw != "" && strings.Contains(msg, pw) {
			return errors.New("database: open failed")
		}
	}
	if host := parsed.Hostname(); host != "" {
		if _, ipErr := netip.ParseAddr(host); ipErr == nil && strings.Contains(msg, host) {
			return errors.New("database: open failed")
		}
	}
	return fmt.Errorf("database: open: %w", err)
}
