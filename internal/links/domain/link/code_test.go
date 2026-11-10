package link

import (
	"errors"
	"strings"
	"testing"
)

func TestNewCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "random base62 length 7", raw: "Ab3xyZ9"},
		{name: "random digits", raw: "0123456"},
		{name: "min custom slug", raw: "abc"},
		{name: "max custom slug", raw: strings.Repeat("a", 64)},
		{name: "hyphen and underscore", raw: "a-b_c"},
		{name: "length 7 with hyphen is a slug", raw: "ab-cd_e"},
		{name: "preserves case", raw: "MySlug"},
		{name: "near reserved apis", raw: "apis"},
		{name: "near reserved health", raw: "health"},
		{name: "near reserved api suffix", raw: "api-v1"},

		{name: "empty", raw: "", wantErr: "length"},
		{name: "too short", raw: "ab", wantErr: "length"},
		{name: "one character", raw: "a", wantErr: "length"},
		{name: "too long", raw: strings.Repeat("a", 65), wantErr: "length"},

		{name: "space", raw: "abc def", wantErr: "charset"},
		{name: "dot", raw: "abc.def", wantErr: "charset"},
		{name: "slash", raw: "abc/def", wantErr: "charset"},
		{name: "question", raw: "abc?def", wantErr: "charset"},
		{name: "newline", raw: "abc\ndef", wantErr: "charset"},
		{name: "non-ascii", raw: "cafés", wantErr: "charset"},

		{name: "reserved api", raw: "api", wantErr: "reserved"},
		{name: "reserved API", raw: "API", wantErr: "reserved"},
		{name: "reserved Api", raw: "Api", wantErr: "reserved"},
		{name: "reserved healthz", raw: "healthz", wantErr: "reserved"},
		{name: "reserved HealthZ", raw: "HealthZ", wantErr: "reserved"},
		{name: "reserved readyz", raw: "readyz", wantErr: "reserved"},
		{name: "reserved READYZ", raw: "READYZ", wantErr: "reserved"},
		{name: "reserved metrics", raw: "metrics", wantErr: "reserved"},
		{name: "reserved METRICS", raw: "METRICS", wantErr: "reserved"},
		{name: "reserved debug", raw: "debug", wantErr: "reserved"},
		{name: "reserved DEBUG", raw: "DEBUG", wantErr: "reserved"},
		{name: "reserved static", raw: "static", wantErr: "reserved"},
		{name: "reserved STATIC", raw: "STATIC", wantErr: "reserved"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewCode(tt.raw)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("NewCode(%q) error = %v", tt.raw, err)
				}
				if got.String() != tt.raw {
					t.Fatalf("String() = %q, want %q", got.String(), tt.raw)
				}
				again, err := NewCode(tt.raw)
				if err != nil {
					t.Fatalf("NewCode again: %v", err)
				}
				if got != again {
					t.Fatalf("codes not equal: %#v vs %#v", got, again)
				}
				return
			}
			if err == nil {
				t.Fatalf("NewCode(%q) = %q, want error %s", tt.raw, got.String(), tt.wantErr)
			}
			if !errors.Is(err, ErrInvalidCode) {
				t.Fatalf("errors.Is(ErrInvalidCode) = false, err = %v", err)
			}
			var invalid *InvalidCodeError
			if !errors.As(err, &invalid) {
				t.Fatalf("errors.As(InvalidCodeError) = false, err = %v", err)
			}
			if invalid.Reason != tt.wantErr {
				t.Fatalf("reason = %q, want %q", invalid.Reason, tt.wantErr)
			}
		})
	}
}

func TestCodeAlphabet(t *testing.T) {
	t.Parallel()

	if len(Base62Alphabet) != 62 {
		t.Fatalf("Base62Alphabet length = %d, want 62", len(Base62Alphabet))
	}
	seen := make(map[byte]struct{}, len(Base62Alphabet))
	for i := 0; i < len(Base62Alphabet); i++ {
		c := Base62Alphabet[i]
		if _, ok := seen[c]; ok {
			t.Fatalf("duplicate alphabet byte %q", c)
		}
		seen[c] = struct{}{}
		sample := "aa" + string(c)
		if _, err := NewCode(sample); err != nil {
			t.Fatalf("NewCode(%q) error = %v", sample, err)
		}
	}
	for _, c := range []byte{'_', '-'} {
		sample := "aa" + string(c)
		if _, err := NewCode(sample); err != nil {
			t.Fatalf("NewCode(%q) error = %v", sample, err)
		}
	}

	random := Base62Alphabet[:RandomCodeLength]
	got, err := NewCode(random)
	if err != nil {
		t.Fatalf("NewCode(%q) error = %v", random, err)
	}
	if got.String() != random || len(got.String()) != RandomCodeLength {
		t.Fatalf("random shape = %q", got.String())
	}

	var zero Code
	if zero.String() != "" {
		t.Fatalf("zero Code String() = %q", zero.String())
	}
}
