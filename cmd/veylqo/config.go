package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jovrion/veylqo/internal/links/app/command"
	"github.com/jovrion/veylqo/internal/links/ports"
	"github.com/jovrion/veylqo/internal/migrate"
)

const (
	defaultAddr            = ":8080"
	defaultRateBurst       = 60
	defaultRateRefill      = time.Second
	defaultShutdownTimeout = 15 * time.Second
	defaultClickCapacity   = 0
	defaultClickBatch      = 0
	defaultClickInterval   = time.Duration(0)
)

// config is process configuration. databaseURL may carry a password.
// This value is not logged. Secrets are environment variables so they
// do not appear in the process argument list. Operational switches are
// flags, with the same names available as environment variables.
type config struct {
	addr            string
	databaseURL     string
	migrate         bool
	allowedHosts    []string
	publishableKey  string
	adminToken      string
	origins         []string
	trustedProxies  string
	burst           int
	refill          time.Duration
	clickCapacity   int
	clickBatch      int
	clickInterval   time.Duration
	shutdownTimeout time.Duration
	addrFile        string
}

func (c config) secrets() []string {
	secrets := make([]string, 0, 4)
	if c.databaseURL != "" {
		secrets = append(secrets, c.databaseURL)
	}
	if c.publishableKey != "" {
		secrets = append(secrets, c.publishableKey)
	}
	if c.adminToken != "" {
		secrets = append(secrets, c.adminToken)
	}
	parsed, err := url.Parse(c.databaseURL)
	if err != nil || parsed == nil || parsed.User == nil {
		return secrets
	}
	if name := parsed.User.Username(); name != "" {
		secrets = append(secrets, name)
	}
	if pw, ok := parsed.User.Password(); ok && pw != "" {
		secrets = append(secrets, pw)
	}
	return secrets
}

func envSecrets(lookup func(string) (string, bool)) []string {
	var secrets []string
	for _, key := range []string{"VEYLQO_DB_URL", "VEYLQO_PUBLISHABLE_KEY", "VEYLQO_ADMIN_TOKEN"} {
		if value, ok := lookup(key); ok && strings.TrimSpace(value) != "" {
			secrets = append(secrets, strings.TrimSpace(value))
		}
	}
	return secrets
}

func redact(msg string, secrets []string) string {
	if msg == "" {
		return "exit"
	}
	for _, secret := range secrets {
		if len(secret) >= 4 && strings.Contains(msg, secret) {
			return "redacted"
		}
	}
	return msg
}

// parseConfig reads flags and the environment. A flag overrides the
// environment for the switches that have both. The database URL and the
// two credentials are environment only. The error text does not include
// a URL, a password, a key, or a token.
func parseConfig(args []string, lookup func(string) (string, bool)) (config, error) {
	if lookup == nil {
		return config{}, errors.New("config: nil environment")
	}
	fs := flag.NewFlagSet("veylqo", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	addr := fs.String("addr", "", "listen address (default from VEYLQO_ADDR or :8080)")
	migrateOn := fs.Bool("migrate", true, "apply embedded migrations on start (VEYLQO_MIGRATE)")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if fs.NArg() != 0 {
		return config{}, errors.New("config: unexpected arguments")
	}

	cfg := config{
		addr:            firstNonEmpty(*addr, getenv(lookup, "VEYLQO_ADDR"), defaultAddr),
		migrate:         *migrateOn,
		burst:           defaultRateBurst,
		refill:          defaultRateRefill,
		shutdownTimeout: defaultShutdownTimeout,
	}
	if visited(fs, "migrate") == "" {
		on, err := envBool(lookup, "VEYLQO_MIGRATE", true)
		if err != nil {
			return config{}, err
		}
		cfg.migrate = on
	}

	db, ok := lookup("VEYLQO_DB_URL")
	if !ok || strings.TrimSpace(db) == "" {
		return config{}, errors.New("config: VEYLQO_DB_URL is required")
	}
	cfg.databaseURL = strings.TrimSpace(db)
	if _, err := dialectOf(cfg.databaseURL); err != nil {
		return config{}, err
	}

	hosts, ok := lookup("VEYLQO_ALLOWED_HOSTS")
	if !ok {
		return config{}, errors.New("config: VEYLQO_ALLOWED_HOSTS is required")
	}
	cfg.allowedHosts = splitList(hosts)
	if _, err := command.NewAllowlist(cfg.allowedHosts); err != nil {
		return config{}, errors.New("config: invalid allowlist host")
	}

	key, ok := lookup("VEYLQO_PUBLISHABLE_KEY")
	if !ok || strings.TrimSpace(key) == "" {
		return config{}, errors.New("config: VEYLQO_PUBLISHABLE_KEY is required")
	}
	cfg.publishableKey = strings.TrimSpace(key)
	token, ok := lookup("VEYLQO_ADMIN_TOKEN")
	if !ok || strings.TrimSpace(token) == "" {
		return config{}, errors.New("config: VEYLQO_ADMIN_TOKEN is required")
	}
	cfg.adminToken = strings.TrimSpace(token)
	if cfg.publishableKey == cfg.adminToken {
		return config{}, errors.New("config: publishable key and admin token must be distinct")
	}

	if origins, ok := lookup("VEYLQO_CORS_ORIGINS"); ok {
		cfg.origins = splitList(origins)
	}
	if proxies, ok := lookup("VEYLQO_TRUSTED_PROXIES"); ok {
		cfg.trustedProxies = proxies
		if _, err := ports.ParseTrustedProxies(proxies); err != nil {
			return config{}, errors.New("config: invalid trusted proxy")
		}
	}
	if file, ok := lookup("VEYLQO_ADDR_FILE"); ok {
		cfg.addrFile = strings.TrimSpace(file)
	}

	burst, err := envInt(lookup, "VEYLQO_RATE_BURST", defaultRateBurst)
	if err != nil {
		return config{}, err
	}
	cfg.burst = burst
	refill, err := envDuration(lookup, "VEYLQO_RATE_REFILL", defaultRateRefill)
	if err != nil {
		return config{}, err
	}
	cfg.refill = refill
	capacity, err := envInt(lookup, "VEYLQO_CLICK_CAPACITY", defaultClickCapacity)
	if err != nil {
		return config{}, err
	}
	cfg.clickCapacity = capacity
	batch, err := envInt(lookup, "VEYLQO_CLICK_BATCH", defaultClickBatch)
	if err != nil {
		return config{}, err
	}
	cfg.clickBatch = batch
	interval, err := envDuration(lookup, "VEYLQO_CLICK_INTERVAL", defaultClickInterval)
	if err != nil {
		return config{}, err
	}
	cfg.clickInterval = interval
	shutdown, err := envDuration(lookup, "VEYLQO_SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	if err != nil {
		return config{}, err
	}
	if shutdown <= 0 {
		return config{}, errors.New("config: shutdown timeout must be positive")
	}
	cfg.shutdownTimeout = shutdown
	return cfg, nil
}

func dialectOf(raw string) (migrate.Dialect, error) {
	scheme, rest, ok := strings.Cut(strings.TrimSpace(raw), "://")
	if !ok || scheme == "" || strings.TrimSpace(rest) == "" {
		return "", errors.New("config: database url scheme required")
	}
	switch strings.ToLower(scheme) {
	case "sqlite":
		return migrate.SQLite, nil
	case "postgres":
		return migrate.Postgres, nil
	default:
		return "", errors.New("config: unsupported database scheme")
	}
}

func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	fields := strings.Split(value, ",")
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		out = append(out, field)
	}
	return out
}

func getenv(lookup func(string) (string, bool), key string) string {
	value, ok := lookup(key)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// visited reports the flag value when the user passed it. An empty
// result means the flag was omitted, so the environment applies.
func visited(fs *flag.FlagSet, name string) string {
	found := ""
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = f.Value.String()
		}
	})
	return found
}

func envBool(lookup func(string) (string, bool), key string, fallback bool) (bool, error) {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, fmt.Errorf("config: invalid %s", key)
	}
	return parsed, nil
}

func envInt(lookup func(string) (string, bool), key string, fallback int) (int, error) {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("config: invalid %s", key)
	}
	return parsed, nil
}

func envDuration(lookup func(string) (string, bool), key string, fallback time.Duration) (time.Duration, error) {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("config: invalid %s", key)
	}
	return parsed, nil
}
