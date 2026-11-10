package link

import (
	"net/url"
	"strings"
)

// KindKeepDPASIN is the per-host path rule that keeps /dp/ASIN and drops
// the rest of the path. It is data for a host list, not a rewrite of
// every host (ADR-0008).
const KindKeepDPASIN = "dp-asin"

// HostRule is a per-host path rule. Hosts are ASCII or punycode. An entry
// matches that host and its subdomains. Kind must be KindKeepDPASIN.
// Rules do not run when the caller passes clean: false.
type HostRule struct {
	Hosts []string
	Kind  string
}

// CleanInput is the cleaner request. Skip is the clean: false option.
// The zero value strips tracking parameters and applies path rules. It
// does not skip scheme checks, fragment dropping, or the private-host
// block. The allowlist is not this service's job.
type CleanInput struct {
	Skip   bool
	Config DestinationConfig
}

// Cleaner strips tracking queries and applies per-host path rules, then
// validates the destination. It does not fetch the URL, follow redirects,
// or convert IDN. The zero Cleaner strips the default query list and has
// no path rules. Use DefaultCleaner for the built-in host rules.
type Cleaner struct {
	rules []hostRule
}

type hostRule struct {
	hosts []string
	kind  string
}

// Default tracking parameters. Names match case-insensitively. utm_ is a
// prefix; the others are exact. The extra names are click identifiers of
// the same class as fbclid (ADR-0008). They are not a general rewriter.
var trackingExact = []string{
	"fbclid",
	"gclid",
	"ref",
	"tag",
	"mc_eid",
	"igshid",
	"msclkid",
	"gbraid",
	"wbraid",
	"twclid",
	"ttclid",
	"dclid",
	"yclid",
}

// defaultHostRules preserves /dp/ASIN on Amazon storefronts. The list is
// data. A host that is not here keeps its path.
var defaultHostRules = []HostRule{
	{
		Hosts: []string{
			"amazon.com",
			"amazon.co.uk",
			"amazon.de",
			"amazon.fr",
			"amazon.es",
			"amazon.it",
			"amazon.ca",
			"amazon.co.jp",
			"amazon.com.au",
			"amazon.in",
		},
		Kind: KindKeepDPASIN,
	},
}

var defaultCleaner = newDefaultCleaner()

func newDefaultCleaner() Cleaner {
	c, err := NewCleaner(defaultHostRules)
	if err != nil {
		return Cleaner{}
	}
	return c
}

// DefaultCleaner returns the cleaner with the built-in host rules.
func DefaultCleaner() Cleaner { return defaultCleaner }

// Clean strips tracking parameters with the default cleaner, then
// validates. Skip on the input is clean: false.
func Clean(raw string, in CleanInput) (Destination, error) {
	return defaultCleaner.Clean(raw, in)
}

// NewCleaner validates per-host rules. A nil or empty slice is a cleaner
// with no path rules. Query stripping still applies. An invalid host or
// an unknown kind is an error. The host text is not included in that error.
func NewCleaner(rules []HostRule) (Cleaner, error) {
	compiled := make([]hostRule, 0, len(rules))
	for _, rule := range rules {
		if len(rule.Hosts) == 0 || rule.Kind == "" {
			return Cleaner{}, &InvalidCleanRuleError{Reason: "empty"}
		}
		if rule.Kind != KindKeepDPASIN {
			return Cleaner{}, &InvalidCleanRuleError{Reason: "kind"}
		}
		hosts := make([]string, 0, len(rule.Hosts))
		for _, host := range rule.Hosts {
			canonical, ok := canonicalRuleHost(host)
			if !ok {
				return Cleaner{}, &InvalidCleanRuleError{Reason: "host"}
			}
			hosts = append(hosts, canonical)
		}
		compiled = append(compiled, hostRule{hosts: hosts, kind: rule.Kind})
	}
	return Cleaner{rules: compiled}, nil
}

// Clean applies this cleaner's rules. When in.Skip is set, parameter
// stripping and path rules are skipped. Scheme, fragment, and private-host
// checks still run via NewDestination. The returned error never includes
// the URL or its query.
func (c Cleaner) Clean(raw string, in CleanInput) (Destination, error) {
	if !in.Skip {
		if cleaned, ok := c.rewrite(raw); ok {
			raw = cleaned
		}
	}
	return NewDestination(raw, in.Config)
}

func (c Cleaner) rewrite(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return "", false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	query := parsed.Query()
	for key := range query {
		if trackingParam(key) {
			query.Del(key)
		}
	}
	parsed.RawQuery = query.Encode()
	parsed.Fragment = ""
	parsed.RawFragment = ""
	if path, ok := c.rewritePath(parsed.Hostname(), parsed.EscapedPath()); ok {
		parsed.Path = path
		parsed.RawPath = ""
	}
	return parsed.String(), true
}

func (c Cleaner) rewritePath(host, path string) (string, bool) {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, rule := range c.rules {
		if !rule.matches(host) || rule.kind != KindKeepDPASIN {
			continue
		}
		if kept, ok := keepDPASIN(path); ok {
			return kept, true
		}
	}
	return "", false
}

func (r hostRule) matches(host string) bool {
	for _, candidate := range r.hosts {
		if host == candidate || strings.HasSuffix(host, "."+candidate) {
			return true
		}
	}
	return false
}

// keepDPASIN reduces a path that contains /dp/<ASIN> to /dp/<ASIN>.
// The ASIN is ten ASCII letters or digits. Its case is preserved. A path
// without that shape is left unchanged.
func keepDPASIN(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(segments); i++ {
		if !strings.EqualFold(segments[i], "dp") || !asin(segments[i+1]) {
			continue
		}
		return "/dp/" + segments[i+1], true
	}
	return "", false
}

func asin(segment string) bool {
	if len(segment) != 10 {
		return false
	}
	for i := 0; i < len(segment); i++ {
		c := segment[i]
		switch {
		case c >= '0' && c <= '9', c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z':
		default:
			return false
		}
	}
	return true
}

func trackingParam(name string) bool {
	name = strings.ToLower(name)
	if strings.HasPrefix(name, "utm_") {
		return true
	}
	for _, exact := range trackingExact {
		if name == exact {
			return true
		}
	}
	return false
}

func canonicalRuleHost(raw string) (string, bool) {
	host := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
	if host == "" || strings.Contains(host, "://") || strings.Contains(host, "/") || strings.Contains(host, ":") || strings.Contains(host, "?") {
		return "", false
	}
	if len(host) > 253 || !asciiHost(host) {
		return "", false
	}
	return host, true
}
