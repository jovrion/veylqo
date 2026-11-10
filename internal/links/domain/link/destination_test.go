package link

import (
	"errors"
	"strings"
	"testing"
)

func TestNewDestination(t *testing.T) {
	t.Parallel()

	blockingOn := DestinationConfig{}
	blockingOff := DestinationConfig{AllowPrivateHosts: true}

	tests := []struct {
		name    string
		raw     string
		cfg     DestinationConfig
		want    string
		wantErr string
	}{
		{name: "https host lowercased", raw: "HTTPS://Example.COM/Path", cfg: blockingOn, want: "https://example.com/Path"},
		{name: "fragment dropped", raw: "https://example.com/a#secret", cfg: blockingOn, want: "https://example.com/a"},
		{name: "empty query omitted", raw: "https://example.com/a?", cfg: blockingOn, want: "https://example.com/a"},
		{name: "query order stable", raw: "https://example.com/a?b=1&a=2", cfg: blockingOn, want: "https://example.com/a?a=2&b=1"},
		{name: "query order already sorted", raw: "https://example.com/a?a=2&b=1", cfg: blockingOn, want: "https://example.com/a?a=2&b=1"},
		{name: "utm kept for the cleaner", raw: "https://example.com/a?utm_source=x&b=1", cfg: blockingOn, want: "https://example.com/a?b=1&utm_source=x"},
		{name: "default http port dropped", raw: "http://example.com:80/a", cfg: blockingOn, want: "http://example.com/a"},
		{name: "default https port dropped", raw: "https://example.com:443/a", cfg: blockingOn, want: "https://example.com/a"},
		{name: "non-default port kept", raw: "https://example.com:8443/a", cfg: blockingOn, want: "https://example.com:8443/a"},
		{name: "trailing dot stripped", raw: "https://Example.COM./a", cfg: blockingOn, want: "https://example.com/a"},
		{name: "punycode host", raw: "https://xn--bcher-kva.example/a", cfg: blockingOn, want: "https://xn--bcher-kva.example/a"},
		{name: "public ip", raw: "http://8.8.8.8/a", cfg: blockingOn, want: "http://8.8.8.8/a"},
		{name: "mapped public ip unmapped", raw: "http://[::ffff:8.8.8.8]/a", cfg: blockingOn, want: "http://8.8.8.8/a"},
		{name: "172 outside private range", raw: "http://172.15.0.1/a", cfg: blockingOn, want: "http://172.15.0.1/a"},

		{name: "javascript", raw: "javascript:alert(1)", cfg: blockingOn, wantErr: "scheme"},
		{name: "javascript mixed case", raw: "JAVASCRIPT:alert(1)", cfg: blockingOn, wantErr: "scheme"},
		{name: "javascript blocking off", raw: "javascript:alert(1)", cfg: blockingOff, wantErr: "scheme"},
		{name: "data", raw: "data:text/html,hi", cfg: blockingOn, wantErr: "scheme"},
		{name: "data blocking off", raw: "data:text/html,hi", cfg: blockingOff, wantErr: "scheme"},
		{name: "file", raw: "file:///etc/passwd", cfg: blockingOn, wantErr: "scheme"},
		{name: "ftp", raw: "ftp://example.com/a", cfg: blockingOn, wantErr: "scheme"},
		{name: "empty", raw: "", cfg: blockingOn, wantErr: "scheme"},
		{name: "relative", raw: "/a", cfg: blockingOn, wantErr: "scheme"},
		{name: "missing host", raw: "http://", cfg: blockingOn, wantErr: "host"},
		{name: "unicode host", raw: "https://exämple.com/a", cfg: blockingOn, wantErr: "host"},
		{name: "leading zero ip", raw: "http://0177.0.0.1/", cfg: blockingOn, wantErr: "host"},
		{name: "short ip", raw: "http://127.1/", cfg: blockingOn, wantErr: "host"},
		{name: "decimal ip", raw: "http://2130706433/", cfg: blockingOn, wantErr: "host"},
		{name: "hex ip", raw: "http://0x7f000001/", cfg: blockingOn, wantErr: "host"},
		{name: "dotted hex ip", raw: "http://0x7f.0.0.1/", cfg: blockingOn, wantErr: "host"},

		{name: "loopback blocking on", raw: "http://127.0.0.1/", cfg: blockingOn, wantErr: "private"},
		{name: "loopback other", raw: "http://127.0.0.2/", cfg: blockingOn, wantErr: "private"},
		{name: "loopback mapped", raw: "http://[::ffff:127.0.0.1]/", cfg: blockingOn, wantErr: "private"},
		{name: "ipv6 loopback", raw: "http://[::1]/", cfg: blockingOn, wantErr: "private"},
		{name: "unspecified v4", raw: "http://0.0.0.0/", cfg: blockingOn, wantErr: "private"},
		{name: "unspecified v6", raw: "http://[::]/", cfg: blockingOn, wantErr: "private"},
		{name: "private 10", raw: "http://10.1.2.3/", cfg: blockingOn, wantErr: "private"},
		{name: "private 192.168", raw: "http://192.168.0.1/", cfg: blockingOn, wantErr: "private"},
		{name: "private 172.16", raw: "http://172.16.5.1/", cfg: blockingOn, wantErr: "private"},
		{name: "link-local v4", raw: "http://169.254.1.1/", cfg: blockingOn, wantErr: "private"},
		{name: "link-local v6", raw: "http://[fe80::1]/", cfg: blockingOn, wantErr: "private"},
		{name: "link-local multicast", raw: "http://224.0.0.1/", cfg: blockingOn, wantErr: "private"},
		{name: "unique local", raw: "http://[fc00::1]/", cfg: blockingOn, wantErr: "private"},
		{name: "localhost", raw: "http://LOCALHOST/a", cfg: blockingOn, wantErr: "private"},
		{name: "localhost subdomain", raw: "http://foo.localhost/a", cfg: blockingOn, wantErr: "private"},
		{name: "loopback with query", raw: "http://127.0.0.1/?token=secret", cfg: blockingOn, wantErr: "private"},

		{name: "loopback blocking off", raw: "http://127.0.0.1/", cfg: blockingOff, want: "http://127.0.0.1/"},
		{name: "localhost blocking off", raw: "http://LOCALHOST/a", cfg: blockingOff, want: "http://localhost/a"},
		{name: "mapped loopback blocking off", raw: "http://[::ffff:127.0.0.1]/", cfg: blockingOff, want: "http://127.0.0.1/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewDestination(tt.raw, tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("NewDestination() error = %v", err)
				}
				if got.String() != tt.want {
					t.Fatalf("String() = %q, want %q", got.String(), tt.want)
				}
				if strings.Contains(got.String(), "#") {
					t.Fatalf("fragment stored in %q", got.String())
				}
				again, err := NewDestination(got.String(), tt.cfg)
				if err != nil {
					t.Fatalf("normalize again: %v", err)
				}
				if got != again {
					t.Fatalf("not idempotent: %q vs %q", got.String(), again.String())
				}
				return
			}
			if err == nil {
				t.Fatalf("NewDestination() = %q, want error %s", got.String(), tt.wantErr)
			}
			if !errors.Is(err, ErrInvalidDestination) {
				t.Fatalf("errors.Is = false, err = %v", err)
			}
			var invalid *InvalidDestinationError
			if !errors.As(err, &invalid) {
				t.Fatalf("errors.As = false, err = %v", err)
			}
			if invalid.Reason != tt.wantErr {
				t.Fatalf("reason = %q, want %q", invalid.Reason, tt.wantErr)
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "alert") || strings.Contains(err.Error(), "token") {
				t.Fatalf("error leaked destination details: %v", err)
			}
		})
	}
}

func TestDestinationHost(t *testing.T) {
	t.Parallel()

	got, err := NewDestination("https://Example.COM:8443/a", DestinationConfig{})
	if err != nil {
		t.Fatalf("NewDestination() error = %v", err)
	}
	if got.Host() != "example.com" {
		t.Fatalf("Host() = %q", got.Host())
	}
	var zero Destination
	if zero.Host() != "" || zero.String() != "" {
		t.Fatalf("zero Destination = %q %q", zero.String(), zero.Host())
	}
}
