package link

import (
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// DestinationConfig selects the private-host check a deployment can turn
// off. The zero value blocks loopback, private, link-local, and
// unspecified hosts. AllowPrivateHosts opts out of that block only.
// Scheme and host-shape checks always apply (ADR-0008, ADR-0010).
type DestinationConfig struct {
	AllowPrivateHosts bool
}

// ErrInvalidDestination is returned when a string is not a storable
// destination. The error text never includes the URL, its query, or an IP.
var ErrInvalidDestination = errors.New("invalid destination")

// InvalidDestinationError is a typed domain failure. Reason is "parse",
// "scheme", "host", or "private".
type InvalidDestinationError struct {
	Reason string
}

func (e *InvalidDestinationError) Error() string {
	if e == nil || e.Reason == "" {
		return ErrInvalidDestination.Error()
	}
	return "invalid destination: " + e.Reason
}

func (e *InvalidDestinationError) Unwrap() error { return ErrInvalidDestination }

// Destination is a normalized http(s) URL. The fragment is not stored.
// Tracking-parameter stripping is the cleaner (VYQ-16), not this type.
// The zero value is invalid and must not be stored.
type Destination struct {
	value string
}

// NewDestination validates and normalizes a destination. The host is
// lowercased, the fragment is dropped, and an empty query is omitted.
// Remaining query parameters are ordered so equal destinations compare
// equal. Unicode hosts are rejected; this package does not convert IDN
// to punycode. It does not resolve names and does not fetch the URL.
func NewDestination(raw string, cfg DestinationConfig) (Destination, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return Destination{}, &InvalidDestinationError{Reason: "parse"}
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return Destination{}, &InvalidDestinationError{Reason: "scheme"}
	}
	if parsed.Host == "" || parsed.Opaque != "" {
		return Destination{}, &InvalidDestinationError{Reason: "host"}
	}
	host, err := canonicalHost(parsed.Hostname(), parsed.Port(), scheme, !cfg.AllowPrivateHosts)
	if err != nil {
		return Destination{}, err
	}
	parsed.Scheme = scheme
	parsed.Host = host
	parsed.Opaque = ""
	parsed.Fragment = ""
	parsed.RawFragment = ""
	parsed.ForceQuery = false
	parsed.RawQuery = parsed.Query().Encode()
	return Destination{value: parsed.String()}, nil
}

// String returns the normalized destination. Callers must not log it
// when it still has a query string.
func (d Destination) String() string { return d.value }

// Host returns the lowercased hostname without a port. The zero
// Destination returns an empty string.
func (d Destination) Host() string {
	if d.value == "" {
		return ""
	}
	parsed, err := url.Parse(d.value)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

func canonicalHost(hostname, port, scheme string, blockPrivate bool) (string, error) {
	hostname = strings.ToLower(strings.TrimSuffix(hostname, "."))
	if hostname == "" || len(hostname) > 253 || !asciiHost(hostname) {
		return "", &InvalidDestinationError{Reason: "host"}
	}
	if !validPort(port) {
		return "", &InvalidDestinationError{Reason: "host"}
	}
	if addr, err := netip.ParseAddr(hostname); err == nil {
		addr = addr.Unmap()
		if blockPrivate && blockedAddr(addr) {
			return "", &InvalidDestinationError{Reason: "private"}
		}
		hostname = addr.String()
	} else if strings.Contains(hostname, ":") || ambiguousIP(hostname) {
		return "", &InvalidDestinationError{Reason: "host"}
	} else if !dnsHost(hostname) {
		return "", &InvalidDestinationError{Reason: "host"}
	} else if blockPrivate && loopbackName(hostname) {
		return "", &InvalidDestinationError{Reason: "private"}
	}
	if port == "" || defaultPort(scheme, port) {
		if strings.Contains(hostname, ":") {
			return "[" + hostname + "]", nil
		}
		return hostname, nil
	}
	return net.JoinHostPort(hostname, port), nil
}

func blockedAddr(addr netip.Addr) bool {
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsUnspecified()
}

func loopbackName(host string) bool {
	return host == "localhost" || host == "localhost.localdomain" || strings.HasSuffix(host, ".localhost")
}

func asciiHost(host string) bool {
	for i := 0; i < len(host); i++ {
		if host[i] > 127 || host[i] == '%' {
			return false
		}
	}
	return true
}

// ambiguousIP reports hostnames that browsers may treat as an address
// even though netip rejects them: hex (0x7f000001), octal, and short
// dotted forms. A letter other than the hex marker 'x' keeps a name
// like example.com on the DNS path.
func ambiguousIP(host string) bool {
	if strings.Contains(host, "0x") {
		return true
	}
	return ipShaped(host)
}

func ipShaped(host string) bool {
	hasLetter := false
	for i := 0; i < len(host); i++ {
		c := host[i]
		switch {
		case c >= 'a' && c <= 'z' && c != 'x':
			// 'x' is the hex-literal marker (0x7f000001), not a DNS letter.
			hasLetter = true
		case c >= '0' && c <= '9', c == '.', c == 'x':
		default:
			return false
		}
	}
	return !hasLetter
}

func dnsHost(host string) bool {
	if strings.Contains(host, ":") {
		return false
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			switch {
			case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-':
			default:
				return false
			}
		}
	}
	return true
}

func validPort(port string) bool {
	if port == "" {
		return true
	}
	if len(port) > 1 && port[0] == '0' {
		return false
	}
	n := 0
	for i := 0; i < len(port); i++ {
		if port[i] < '0' || port[i] > '9' {
			return false
		}
		n = n*10 + int(port[i]-'0')
		if n > 65535 {
			return false
		}
	}
	return n > 0
}

func defaultPort(scheme, port string) bool {
	return (scheme == "http" && port == "80") || (scheme == "https" && port == "443")
}
