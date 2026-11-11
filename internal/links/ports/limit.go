package ports

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/common/httpx"
)

const defaultMaxBuckets = 8192

// ErrInvalidLimit means the token bucket was not given a positive burst
// and refill. The text has no address.
var ErrInvalidLimit = errString("rate limit burst and refill must be positive")

// ErrInvalidTrustedProxy means a VEYLQO_TRUSTED_PROXIES entry is not an
// IP or a CIDR. The text does not include the entry.
var ErrInvalidTrustedProxy = errString("invalid trusted proxy")

// LimitConfig is the per-IP token bucket (ADR-0010). Buckets are in
// memory for this process. They are not click analytics and they are not
// shared across replicas.
type LimitConfig struct {
	// Burst is the bucket size. It must be at least 1.
	Burst int
	// Refill is the time that adds one token. It must be positive.
	Refill time.Duration
	// MaxBuckets caps the map. Zero uses the default. A full map drops
	// the least recently seen bucket. The dropped address is not logged.
	MaxBuckets int
	// TrustedProxies are the peers allowed to supply X-Forwarded-For.
	// Empty means every forwarded header is ignored.
	TrustedProxies []netip.Prefix
	// Now is the clock. Nil means time.Now. It must not log.
	Now func() time.Time
}

// Limiter is a per-IP token bucket. It does not write a log.
type Limiter struct {
	mu         sync.Mutex
	buckets    map[netip.Addr]*bucket
	burst      float64
	refill     time.Duration
	maxBuckets int
	proxies    []netip.Prefix
	now        func() time.Time
}

type bucket struct {
	tokens float64
	seen   time.Time
}

// NewLimiter validates the bucket. It does not read the environment.
// Pass ParseTrustedProxies(os.Getenv("VEYLQO_TRUSTED_PROXIES")) from
// composition, not from this package.
func NewLimiter(cfg LimitConfig) (*Limiter, error) {
	if cfg.Burst < 1 || cfg.Refill <= 0 {
		return nil, ErrInvalidLimit
	}
	maxBuckets := cfg.MaxBuckets
	if maxBuckets == 0 {
		maxBuckets = defaultMaxBuckets
	}
	if maxBuckets < 1 {
		return nil, ErrInvalidLimit
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Limiter{
		buckets:    make(map[netip.Addr]*bucket),
		burst:      float64(cfg.Burst),
		refill:     cfg.Refill,
		maxBuckets: maxBuckets,
		proxies:    append([]netip.Prefix(nil), cfg.TrustedProxies...),
		now:        now,
	}, nil
}

// ParseTrustedProxies parses VEYLQO_TRUSTED_PROXIES. The value is a
// comma-separated list of IPs or CIDRs. Empty trusts no proxy, so
// forwarded headers are ignored. A single IP is a /32 or /128. The
// error does not include the entry.
func ParseTrustedProxies(value string) ([]netip.Prefix, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	fields := strings.Split(value, ",")
	proxies := make([]netip.Prefix, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		prefix, err := parseProxy(field)
		if err != nil {
			return nil, err
		}
		proxies = append(proxies, prefix)
	}
	return proxies, nil
}

func parseProxy(raw string) (netip.Prefix, error) {
	if strings.Contains(raw, "/") {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil || !prefix.IsValid() {
			return netip.Prefix{}, ErrInvalidTrustedProxy
		}
		return prefix.Masked(), nil
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Prefix{}, ErrInvalidTrustedProxy
	}
	addr = normalizeAddr(addr)
	if !addr.IsValid() {
		return netip.Prefix{}, ErrInvalidTrustedProxy
	}
	return netip.PrefixFrom(addr, addr.BitLen()).Masked(), nil
}

// Handler limits next. Excess is 429 problem+json. The body does not
// include the client address. A nil Limiter fails closed.
func (l *Limiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(r) {
			httpx.WriteProblem(w, r, apperrors.NewRateLimited("rate limited", nil))
			return
		}
		if next == nil {
			httpx.WriteProblem(w, r, errUnavailable)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *Limiter) allow(r *http.Request) bool {
	if l == nil || r == nil {
		return false
	}
	addr, ok := clientIP(r, l.proxies)
	if !ok {
		return false
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.take(addr, now)
}

func (l *Limiter) take(addr netip.Addr, now time.Time) bool {
	b := l.buckets[addr]
	if b == nil {
		if len(l.buckets) >= l.maxBuckets {
			l.evict()
		}
		b = &bucket{tokens: l.burst, seen: now}
		l.buckets[addr] = b
	} else if elapsed := now.Sub(b.seen); elapsed > 0 {
		b.tokens += float64(elapsed) / float64(l.refill)
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.seen = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (l *Limiter) evict() {
	var oldest netip.Addr
	var seen time.Time
	first := true
	for addr, b := range l.buckets {
		if b == nil {
			delete(l.buckets, addr)
			return
		}
		if first || b.seen.Before(seen) {
			oldest = addr
			seen = b.seen
			first = false
		}
	}
	if !first {
		delete(l.buckets, oldest)
	}
}

// clientIP buckets on the connection address unless that peer is a
// trusted proxy. Only then may X-Forwarded-For choose the client.
// Untrusted forwarded headers are ignored. Nothing here is logged.
func clientIP(r *http.Request, proxies []netip.Prefix) (netip.Addr, bool) {
	peer, ok := peerAddr(r.RemoteAddr)
	if !ok {
		return netip.Addr{}, false
	}
	if !prefixContains(proxies, peer) {
		return peer, true
	}
	header := ""
	if r.Header != nil {
		header = r.Header.Get("X-Forwarded-For")
	}
	if addr, ok := forwardedClient(header, proxies); ok {
		return addr, true
	}
	return peer, true
}

func peerAddr(remote string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	addr = normalizeAddr(addr)
	if !addr.IsValid() {
		return netip.Addr{}, false
	}
	return addr, true
}

func forwardedClient(header string, proxies []netip.Prefix) (netip.Addr, bool) {
	if strings.TrimSpace(header) == "" {
		return netip.Addr{}, false
	}
	parts := strings.Split(header, ",")
	chain := make([]netip.Addr, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), `"`)
		if part == "" {
			continue
		}
		addr, ok := parseForwardedAddr(part)
		if !ok {
			return netip.Addr{}, false
		}
		chain = append(chain, addr)
	}
	if len(chain) == 0 {
		return netip.Addr{}, false
	}
	for i := len(chain) - 1; i >= 0; i-- {
		if !prefixContains(proxies, chain[i]) {
			return chain[i], true
		}
	}
	return chain[0], true
}

func parseForwardedAddr(raw string) (netip.Addr, bool) {
	if addr, err := netip.ParseAddr(raw); err == nil {
		addr = normalizeAddr(addr)
		return addr, addr.IsValid()
	}
	host, _, err := net.SplitHostPort(raw)
	if err != nil {
		return netip.Addr{}, false
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	addr = normalizeAddr(addr)
	return addr, addr.IsValid()
}

func prefixContains(proxies []netip.Prefix, addr netip.Addr) bool {
	addr = normalizeAddr(addr)
	if !addr.IsValid() {
		return false
	}
	for _, prefix := range proxies {
		if prefix.IsValid() && prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func normalizeAddr(addr netip.Addr) netip.Addr {
	if !addr.IsValid() {
		return addr
	}
	addr = addr.Unmap()
	if addr.Zone() != "" {
		addr = addr.WithZone("")
	}
	return addr
}

// errString is a sentinel. The text is fixed at the declaration and
// does not include a request value.
type errString string

func (e errString) Error() string { return string(e) }
