package command

import (
	"errors"
	"net/netip"
	"strings"
)

// ErrInvalidAllowlist is returned when an allowlist entry is not a
// hostname or an IP literal. The error text does not include the entry,
// which may be an address.
var ErrInvalidAllowlist = errors.New("invalid allowlist host")

// Allowlist is the shorten-time host policy (ADR-0010). An empty list
// fails closed: Allows is false for every host. Matching is exact after
// ASCII lowercasing. Subdomains are not implied. This is not a domain
// rule and it does not fetch the host.
type Allowlist struct {
	hosts map[string]struct{}
}

// NewAllowlist validates operator hosts. An empty or nil slice is a
// closed list, not an error. Entries are hostnames or IP literals, ASCII
// or punycode, without a scheme, port, or path. A Unicode label is
// rejected rather than converted.
func NewAllowlist(hosts []string) (Allowlist, error) {
	set := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		key, err := canonicalAllowHost(host)
		if err != nil {
			return Allowlist{}, err
		}
		set[key] = struct{}{}
	}
	return Allowlist{hosts: set}, nil
}

// Allows reports whether host may be stored. A zero Allowlist denies
// every host. Comparison does not resolve DNS.
func (a Allowlist) Allows(host string) bool {
	if len(a.hosts) == 0 {
		return false
	}
	key, err := canonicalAllowHost(host)
	if err != nil {
		return false
	}
	_, ok := a.hosts[key]
	return ok
}

func canonicalAllowHost(raw string) (string, error) {
	host := strings.TrimSpace(raw)
	host = strings.TrimSuffix(host, ".")
	if host == "" || strings.Contains(host, "://") || strings.ContainsAny(host, "/?#@ \t") {
		return "", ErrInvalidAllowlist
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") && len(host) > 2 {
		host = host[1 : len(host)-1]
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.String(), nil
	}
	host = strings.ToLower(host)
	if strings.Contains(host, ":") || strings.Contains(host, "*") || !asciiName(host) {
		return "", ErrInvalidAllowlist
	}
	return host, nil
}

func asciiName(host string) bool {
	if host == "" || len(host) > 253 || strings.Contains(host, "..") {
		return false
	}
	for i := 0; i < len(host); i++ {
		c := host[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '.', c == '-':
		default:
			return false
		}
	}
	return true
}
