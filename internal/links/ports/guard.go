package ports

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"

	json "encoding/json/v2"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/common/httpx"
)

const (
	routePublic = iota
	routeShorten
	routeAdmin
)

// ErrKeyRequired means the publishable key was empty. Shorten fails
// closed. The text is not a credential.
var ErrKeyRequired = errString("publishable key is required")

// ErrAdminTokenRequired means the admin token was empty. Admin routes
// fail closed.
var ErrAdminTokenRequired = errString("admin token is required")

// ErrKeysNotDistinct means the two credentials were the same string.
// The text does not include either value.
var ErrKeysNotDistinct = errString("publishable key and admin token must be distinct")

// ErrInvalidOrigin means a CORS origin is not an http or https
// scheme://host[:port]. The text does not include the value.
var ErrInvalidOrigin = errString("invalid CORS origin")

// APIGuardConfig is the abuse edge for the link API (ADR-0010).
// Composition in cmd or service supplies the values. This package does
// not read the environment.
type APIGuardConfig struct {
	// PublishableKey authorizes POST /api/v1/links only.
	PublishableKey string
	// AdminToken authorizes GET and DELETE of a link, and stats.
	// It must differ from PublishableKey.
	AdminToken string
	// Origins are the only browser origins that receive CORS headers.
	// Empty allows none. * is rejected. A host is lowercased. The
	// header is reflected only when it already equals that form, never as *.
	Origins []string
	// Limit is the per-IP bucket for this guard.
	Limit LimitConfig
}

type apiGuard struct {
	next        http.Handler
	publishable string
	admin       string
	origins     map[string]struct{}
	limiter     *Limiter
	csrf        *http.CrossOriginProtection
}

// NewAPIGuard returns middleware for the link API mux.
//
// Order is CORS headers, per-IP limit, credential check, then
// http.CrossOriginProtection on admin routes only. POST /api/v1/links
// is not wrapped in CrossOriginProtection: the publishable key is a
// bearer header sent by a page on another origin. GET /{code} is not
// an admin route and does not require a key. Missing or wrong
// credentials are 401 problem+json. Excess requests are 429.
func NewAPIGuard(cfg APIGuardConfig) (func(http.Handler) http.Handler, error) {
	if cfg.PublishableKey == "" {
		return nil, ErrKeyRequired
	}
	if cfg.AdminToken == "" {
		return nil, ErrAdminTokenRequired
	}
	if tokensEqual(cfg.PublishableKey, cfg.AdminToken) {
		return nil, ErrKeysNotDistinct
	}
	origins, err := parseOrigins(cfg.Origins)
	if err != nil {
		return nil, err
	}
	limiter, err := NewLimiter(cfg.Limit)
	if err != nil {
		return nil, err
	}
	csrf := http.NewCrossOriginProtection()
	for origin := range origins {
		if err := csrf.AddTrustedOrigin(origin); err != nil {
			return nil, ErrInvalidOrigin
		}
	}
	csrf.SetDenyHandler(http.HandlerFunc(writeCrossOriginProblem))
	return func(next http.Handler) http.Handler {
		return &apiGuard{
			next:        next,
			publishable: cfg.PublishableKey,
			admin:       cfg.AdminToken,
			origins:     origins,
			limiter:     limiter,
			csrf:        csrf,
		}
	}, nil
}

func (g *apiGuard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if g == nil || g.limiter == nil || g.csrf == nil {
		httpx.WriteProblem(w, r, errUnavailable)
		return
	}
	g.noteOrigin(w, r)
	if !g.limiter.allow(r) {
		httpx.WriteProblem(w, r, apperrors.NewRateLimited("rate limited", nil))
		return
	}
	if g.preflight(w, r) {
		return
	}
	switch classify(r) {
	case routeShorten:
		if !bearerMatch(r, g.publishable) {
			writeUnauthorized(w, r)
			return
		}
		g.serve(w, r)
	case routeAdmin:
		if !bearerMatch(r, g.admin) {
			writeUnauthorized(w, r)
			return
		}
		g.csrf.Handler(http.HandlerFunc(g.serve)).ServeHTTP(w, r)
	default:
		g.serve(w, r)
	}
}

func (g *apiGuard) serve(w http.ResponseWriter, r *http.Request) {
	if g.next == nil {
		httpx.WriteProblem(w, r, errUnavailable)
		return
	}
	g.next.ServeHTTP(w, r)
}

func (g *apiGuard) noteOrigin(w http.ResponseWriter, r *http.Request) {
	if w == nil || r == nil {
		return
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	w.Header().Add("Vary", "Origin")
	allowed, ok := g.allowedOrigin(origin)
	if !ok {
		return
	}
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", allowed)
	h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	h.Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	h.Set("Access-Control-Max-Age", "600")
}

func (g *apiGuard) allowedOrigin(origin string) (string, bool) {
	canonical, err := canonicalOrigin(origin)
	if err != nil {
		return "", false
	}
	if _, ok := g.origins[canonical]; !ok {
		return "", false
	}
	// Reflect only the canonical origin. That matches a browser origin
	// and never echoes a header that merely parsed to the same host.
	if strings.TrimSpace(origin) != canonical {
		return "", false
	}
	return canonical, true
}

func (g *apiGuard) preflight(w http.ResponseWriter, r *http.Request) bool {
	if r == nil || r.Method != http.MethodOptions || w == nil {
		return false
	}
	if _, ok := g.allowedOrigin(r.Header.Get("Origin")); !ok {
		return false
	}
	w.Header().Set("Cache-Control", "private, max-age=0")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNoContent)
	return true
}

func classify(r *http.Request) int {
	if r == nil || r.URL == nil {
		return routePublic
	}
	path := r.URL.Path
	switch r.Method {
	case http.MethodPost:
		if path == "/api/v1/links" {
			return routeShorten
		}
	case http.MethodGet, http.MethodDelete:
		if isAdminPath(path) {
			return routeAdmin
		}
	}
	return routePublic
}

func isAdminPath(path string) bool {
	const prefix = "/api/v1/links/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	rest := path[len(prefix):]
	if rest == "" || strings.HasPrefix(rest, "/") {
		return false
	}
	code, extra, ok := strings.Cut(rest, "/")
	if code == "" {
		return false
	}
	if !ok {
		return true
	}
	return extra == "stats"
}

func bearerMatch(r *http.Request, want string) bool {
	got := ""
	if r != nil {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if ok {
			got = token
		}
	}
	return tokensEqual(got, want)
}

func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	if token == "" || strings.ContainsAny(token, " \t\r\n") {
		return "", false
	}
	return token, true
}

func tokensEqual(got, want string) bool {
	if want == "" {
		return false
	}
	sumGot := sha256.Sum256([]byte(got))
	sumWant := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(sumGot[:], sumWant[:]) == 1
}

func parseOrigins(list []string) (map[string]struct{}, error) {
	set := make(map[string]struct{}, len(list))
	for _, raw := range list {
		origin, err := canonicalOrigin(raw)
		if err != nil {
			return nil, err
		}
		set[origin] = struct{}{}
	}
	return set, nil
}

func canonicalOrigin(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" || strings.ContainsAny(raw, "?#\r\n\x00") {
		return "", ErrInvalidOrigin
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.Opaque != "" || parsed.User != nil {
		return "", ErrInvalidOrigin
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrInvalidOrigin
	}
	if parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrInvalidOrigin
	}
	return parsed.Scheme + "://" + strings.ToLower(parsed.Host), nil
}

func writeUnauthorized(w http.ResponseWriter, r *http.Request) {
	if w == nil {
		return
	}
	w.Header().Set("WWW-Authenticate", "Bearer")
	httpx.WriteProblem(w, r, apperrors.NewUnauthorized("credential required", nil))
}

type crossOriginProblem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance,omitempty"`
}

func writeCrossOriginProblem(w http.ResponseWriter, r *http.Request) {
	if w == nil {
		return
	}
	body := crossOriginProblem{
		Type:     "about:blank",
		Title:    http.StatusText(http.StatusForbidden),
		Status:   http.StatusForbidden,
		Detail:   "cross-origin request rejected",
		Instance: requestInstance(r),
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		encoded = []byte(`{"type":"about:blank","title":"Forbidden","status":403,"detail":"cross-origin request rejected"}`)
	}
	w.Header().Set("Content-Type", httpx.ProblemMediaType)
	w.Header().Set("Cache-Control", "private, max-age=0")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write(encoded)
}

func requestInstance(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	path := r.URL.Path
	if path == "" || strings.Contains(path, "?") || strings.Contains(path, "://") {
		return ""
	}
	return path
}
