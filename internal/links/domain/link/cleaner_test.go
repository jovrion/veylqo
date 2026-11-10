package link

import (
	"errors"
	"strings"
	"testing"
)

func TestClean(t *testing.T) {
	t.Parallel()

	blocking := DestinationConfig{}

	tests := []struct {
		name    string
		raw     string
		in      CleanInput
		want    string
		wantErr string
	}{
		{
			name: "utm_source stripped",
			raw:  "https://example.com/a?utm_source=campaign&b=1",
			want: "https://example.com/a?b=1",
		},
		{
			name: "utm prefix and case",
			raw:  "https://example.com/a?UTM_MEDIUM=x&Utm_Campaign=y&b=1",
			want: "https://example.com/a?b=1",
		},
		{
			name: "click ids stripped",
			raw:  "https://example.com/a?fbclid=1&gclid=2&mc_eid=3&igshid=4&msclkid=5&gbraid=6&wbraid=7&twclid=8&ttclid=9&dclid=10&yclid=11&b=1",
			want: "https://example.com/a?b=1",
		},
		{
			name: "ref and tag exact",
			raw:  "https://example.com/a?ref=home&tag=abc&referrer=keep&tagged=keep",
			want: "https://example.com/a?referrer=keep&tagged=keep",
		},
		{
			name: "xutm_source kept",
			raw:  "https://example.com/a?xutm_source=keep&utm_source=drop",
			want: "https://example.com/a?xutm_source=keep",
		},
		{
			name: "remaining query ordered",
			raw:  "https://example.com/a?b=1&a=2&utm_source=x",
			want: "https://example.com/a?a=2&b=1",
		},
		{
			name: "empty query omitted",
			raw:  "https://example.com/a?utm_source=only",
			want: "https://example.com/a",
		},
		{
			name: "fragment not copied back",
			raw:  "https://example.com/a?utm_source=x#secret",
			want: "https://example.com/a",
		},
		{
			name: "two dirty urls meet",
			raw:  "https://Example.COM/a?utm_medium=two&b=1",
			want: "https://example.com/a?b=1",
		},
		{
			name: "clean false keeps tracking",
			raw:  "https://example.com/a?utm_source=campaign&b=1#frag",
			in:   CleanInput{Skip: true},
			want: "https://example.com/a?b=1&utm_source=campaign",
		},
		{
			name: "clean false still drops fragment",
			raw:  "https://example.com/a#secret",
			in:   CleanInput{Skip: true},
			want: "https://example.com/a",
		},
		{
			name:    "javascript still rejected",
			raw:     "javascript:alert(1)",
			wantErr: "scheme",
		},
		{
			name:    "javascript with cleaning off",
			raw:     "javascript:alert(1)?utm_source=x",
			in:      CleanInput{Skip: true},
			wantErr: "scheme",
		},
		{
			name:    "loopback still rejected",
			raw:     "http://127.0.0.1/a?utm_source=secret",
			in:      CleanInput{Config: blocking},
			wantErr: "private",
		},
		{
			name:    "loopback with cleaning off",
			raw:     "http://127.0.0.1/a?utm_source=secret",
			in:      CleanInput{Skip: true, Config: blocking},
			wantErr: "private",
		},
		{
			name: "amazon dp asin kept",
			raw:  "https://www.amazon.com/Kindle-Paperwhite/dp/B00OQVZDJM/ref=sr_1_1?tag=shop-20&utm_source=x#reviews",
			want: "https://www.amazon.com/dp/B00OQVZDJM",
		},
		{
			name: "amazon country host",
			raw:  "https://www.amazon.co.uk/Name/dp/B00OQVZDJM?tag=shop-20",
			want: "https://www.amazon.co.uk/dp/B00OQVZDJM",
		},
		{
			name: "non amazon dp path kept",
			raw:  "https://example.com/Name/dp/B00OQVZDJM?utm_source=x",
			want: "https://example.com/Name/dp/B00OQVZDJM",
		},
		{
			name: "amazon without asin keeps path",
			raw:  "https://www.amazon.com/s?k=kindle&utm_source=x",
			want: "https://www.amazon.com/s?k=kindle",
		},
		{
			name: "clean false skips amazon path rule",
			raw:  "https://www.amazon.com/Name/dp/B00OQVZDJM?utm_source=x&tag=shop-20",
			in:   CleanInput{Skip: true},
			want: "https://www.amazon.com/Name/dp/B00OQVZDJM?tag=shop-20&utm_source=x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Clean(tt.raw, tt.in)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("Clean() succeeded, want error")
				}
				if !errors.Is(err, ErrInvalidDestination) {
					t.Fatalf("errors.Is = false, err = %v", err)
				}
				var invalid *InvalidDestinationError
				if !errors.As(err, &invalid) || invalid.Reason != tt.wantErr {
					t.Fatalf("reason = %v, want %s", err, tt.wantErr)
				}
				if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "alert") || strings.Contains(err.Error(), "127.0.0.1") {
					t.Fatalf("error leaked destination details: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Clean() error = %v", err)
			}
			if got.String() != tt.want {
				t.Fatalf("Clean() mismatch for %s", tt.name)
			}
		})
	}

	again, err := Clean("https://example.com/a?b=1&utm_source=one", CleanInput{})
	if err != nil {
		t.Fatalf("second Clean() error = %v", err)
	}
	if again.String() != "https://example.com/a?b=1" {
		t.Fatal("campaign query was not stripped")
	}
}

func TestCleanHostRule(t *testing.T) {
	t.Parallel()

	cleaner, err := NewCleaner([]HostRule{{
		Hosts: []string{"Shop.Example."},
		Kind:  KindKeepDPASIN,
	}})
	if err != nil {
		t.Fatalf("NewCleaner() error = %v", err)
	}
	got, err := cleaner.Clean("https://www.shop.example/item/dp/B00OQVZDJM?utm_source=x", CleanInput{})
	if err != nil {
		t.Fatalf("Clean() error = %v", err)
	}
	if got.String() != "https://www.shop.example/dp/B00OQVZDJM" {
		t.Fatal("custom host rule did not keep /dp/ASIN")
	}

	other, err := cleaner.Clean("https://example.com/item/dp/B00OQVZDJM?utm_source=x", CleanInput{})
	if err != nil {
		t.Fatalf("Clean() error = %v", err)
	}
	if other.String() != "https://example.com/item/dp/B00OQVZDJM" {
		t.Fatal("rule rewrote a host it does not match")
	}

	skipped, err := cleaner.Clean("https://shop.example/item/dp/B00OQVZDJM?utm_source=x", CleanInput{Skip: true})
	if err != nil {
		t.Fatalf("Clean() error = %v", err)
	}
	if skipped.String() != "https://shop.example/item/dp/B00OQVZDJM?utm_source=x" {
		t.Fatal("clean: false applied a path rule or stripped a query")
	}
}

func TestNewCleanerRejectsRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rules  []HostRule
		reason string
	}{
		{name: "empty", rules: []HostRule{{}}, reason: "empty"},
		{name: "kind", rules: []HostRule{{Hosts: []string{"example.com"}, Kind: "rewrite-all"}}, reason: "kind"},
		{name: "unicode host", rules: []HostRule{{Hosts: []string{"bücher.example"}, Kind: KindKeepDPASIN}}, reason: "host"},
		{name: "url host", rules: []HostRule{{Hosts: []string{"https://example.com/a?token=secret"}, Kind: KindKeepDPASIN}}, reason: "host"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewCleaner(tt.rules)
			if !errors.Is(err, ErrInvalidCleanRule) {
				t.Fatalf("NewCleaner() = %v, want invalid rule", err)
			}
			var invalid *InvalidCleanRuleError
			if !errors.As(err, &invalid) || invalid.Reason != tt.reason {
				t.Fatalf("reason = %v, want %s", err, tt.reason)
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "token") {
				t.Fatalf("error leaked rule details: %v", err)
			}
		})
	}

	if _, err := NewCleaner(nil); err != nil {
		t.Fatalf("NewCleaner(nil) error = %v", err)
	}
}
